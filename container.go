package bender

import (
	"fmt"
	"reflect"
	"sync"
)

type Container struct {
	root     *Container
	parent   *Container
	bindings map[Key]binding
	logger   Logger

	singletonMu sync.Mutex
	singleton   map[Key]any
	inflight    map[Key]*singletonInflight

	scopedMu sync.RWMutex
	scoped   map[Key]any

	lifecycle *lifecycleManager
}

type resolveState struct {
	stack []Key
	seen  map[Key]bool
}

type singletonInflight struct {
	wg  sync.WaitGroup
	val any
	err error
}

type ContainerOption func(*Container)

// WithLogger sets a custom logger for the container.
func WithLogger(logger Logger) ContainerOption {
	return func(c *Container) {
		c.logger = logger
	}
}

// WithLogLevel sets the log level for the default logger.
func WithLogLevel(level LogLevel) ContainerOption {
	return func(c *Container) {
		c.logger = NewDefaultLogger(level)
	}
}

// WithDebug enables debug logging (same as WithLogLevel(LogLevelDebug)).
func WithDebug() ContainerOption {
	return WithLogLevel(LogLevelDebug)
}

// WithInfo enables info logging.
func WithInfo() ContainerOption {
	return WithLogLevel(LogLevelInfo)
}

func New(mods ...Module) *Container {
	return NewWithOptions(nil, mods...)
}

func NewWithOptions(opts []ContainerOption, mods ...Module) *Container {
	temp := &Container{
		logger: loggerFromEnv(),
	}

	for _, opt := range opts {
		opt(temp)
	}

	var b *Binder
	if temp.logger != (NoopLogger{}) {
		b = NewBinderWithLogger(temp.logger)
	} else {
		b = NewBinder()
	}

	b.Install(mods...)

	root := &Container{
		bindings:  b.bindings,
		singleton: make(map[Key]any),
		inflight:  make(map[Key]*singletonInflight),
		scoped:    make(map[Key]any),
		lifecycle: newLifecycleManager(),
		logger:    temp.logger,
	}

	root.root = root

	containerKey := keyOfType(typeOf[*Container](), "")
	root.bindings[containerKey] = binding{
		key:      containerKey,
		scope:    SingletonScope(),
		provider: instanceProvider{v: root},
	}

	root.logger.Info("Container.NewWithOptions: Created container with %d bindings", len(b.bindings))

	if root.logger != (NoopLogger{}) {
		root.logger.Debug("Container.NewWithOptions: Registered bindings:")
		for k, bd := range root.bindings {
			root.logger.Debug("Container.NewWithOptions:   - %s [scope=%s]", formatKey(k), bd.scope)
		}
	}

	return root
}

// Resolve resolves a dependency by type.
// NOTE: Go does not support generic methods, so this is a generic function.
func Resolve[T any](c *Container) (T, error) {
	var zero T

	st := &resolveState{
		seen: make(map[Key]bool),
	}

	k := keyOfType(typeOf[T](), "")

	c.logger.Debug("Container.Resolve: Requesting instance for %s", formatKey(k))

	v, err := c.resolve(k, st)
	if err != nil {
		return zero, err
	}

	c.logger.Debug("Container.Resolve: Successfully resolved %s", formatKey(k))
	return v.(T), nil
}

// ResolveNamed resolves a dependency by type and name.
func ResolveNamed[T any](c *Container, name string) (T, error) {
	var zero T

	st := &resolveState{
		seen: make(map[Key]bool),
	}

	k := keyOfType(typeOf[T](), name)

	c.logger.Debug("Container.ResolveNamed: Requesting instance for %s", formatKey(k))

	v, err := c.resolve(k, st)
	if err != nil {
		return zero, err
	}

	c.logger.Debug("Container.ResolveNamed: Successfully resolved %s", formatKey(k))
	return v.(T), nil
}

func MustResolve[T any](c *Container) T {
	v, err := Resolve[T](c)
	if err != nil {
		panic(err)
	}
	return v
}

func MustResolveNamed[T any](c *Container, name string) T {
	v, err := ResolveNamed[T](c, name)
	if err != nil {
		panic(err)
	}
	return v
}

// CreateScope creates a new scoped container (child).
// This is the primary way to create request-scoped or custom-scoped containers.
func (c *Container) CreateScope() *Container {
	c.logger.Debug("Container.CreateScope: Creating new scoped container")
	return &Container{
		root:      c.root,
		parent:    c,
		bindings:  c.bindings,
		singleton: c.root.singleton,  // shared singleton cache
		scoped:    make(map[Key]any), // new scoped cache
		lifecycle: newLifecycleManager(),
		logger:    c.logger,
	}
}

// Dispose releases all disposable instances managed by this container.
// Should be called when the scope is done (e.g., end of request).
func (c *Container) Dispose() error {
	if c.lifecycle != nil {
		c.logger.Debug("Container.Dispose: Disposing scoped instances")
		return c.lifecycle.dispose()
	}
	return nil
}

// NewTransientScope returns a child container.
// Deprecated: Use CreateScope() instead.
func (c *Container) NewTransientScope() *Container {
	c.logger.Debug("Container.NewTransientScope: Creating new transient scope (deprecated, use CreateScope)")
	return c.CreateScope()
}

// NewSingletonScope also returns a child container.
// Deprecated: Use CreateScope() instead.
func (c *Container) NewSingletonScope() *Container {
	c.logger.Debug("Container.NewSingletonScope: Creating new singleton scope (deprecated, use CreateScope)")
	return c.CreateScope()
}

func (c *Container) resolve(k Key, st *resolveState) (any, error) {
	if st.seen[k] {
		cycle := append([]Key{}, st.stack...)
		cycle = append(cycle, k)
		c.logger.Error("Container.resolve: Dependency cycle detected: %s", formatPath(cycle))
		return nil, CycleError{Cycle: cycle}
	}

	st.seen[k] = true
	st.stack = append(st.stack, k)
	defer func() {
		st.stack = st.stack[:len(st.stack)-1]
		delete(st.seen, k)
	}()

	b, ok := c.bindings[k]
	if !ok {
		path := append([]Key{}, st.stack[:len(st.stack)-1]...)
		c.logger.Error("Container.resolve: Missing binding for %s, path: %s", formatKey(k), formatPath(path))
		return nil, MissingBindingError{Key: k, Path: path}
	}

	c.logger.Debug("Container.resolve: Resolving dependency %s [scope=%s, depth=%d]", formatKey(k), b.scope, len(st.stack))

	switch {
	case isBuiltinSingleton(b.scope):
		return c.getOrCreateSingleton(b, st)
	case isBuiltinTransient(b.scope):
		return c.createTransient(b, st)
	default:
		return c.getOrCreateScoped(b, st)
	}
}

func (c *Container) getOrCreateSingleton(b binding, st *resolveState) (any, error) {
	c.root.singletonMu.Lock()
	if v, ok := c.root.singleton[b.key]; ok {
		c.root.singletonMu.Unlock()
		c.logger.Debug("Container.getOrCreateSingleton: Using cached singleton for %s", formatKey(b.key))
		return v, nil
	}
	if inProgress, ok := c.root.inflight[b.key]; ok {
		c.root.singletonMu.Unlock()
		c.logger.Debug("Container.getOrCreateSingleton: Waiting for concurrent singleton initialization for %s", formatKey(b.key))
		inProgress.wg.Wait()
		if inProgress.err != nil {
			return nil, inProgress.err
		}
		return inProgress.val, nil
	}
	inProgress := &singletonInflight{}
	inProgress.wg.Add(1)
	c.root.inflight[b.key] = inProgress
	c.root.singletonMu.Unlock()

	c.logger.Info("Container.getOrCreateSingleton: Creating new singleton for %s", formatKey(b.key))
	v, err := b.provider.Provide(c, st)

	c.root.singletonMu.Lock()
	delete(c.root.inflight, b.key)
	if err == nil {
		c.root.singleton[b.key] = v
	}
	inProgress.val = v
	inProgress.err = err
	inProgress.wg.Done()
	c.root.singletonMu.Unlock()

	if err != nil {
		c.logger.Error("Container.getOrCreateSingleton: Failed to create singleton %s: %v", formatKey(b.key), err)
		return nil, err
	}

	c.logger.Info("Container.getOrCreateSingleton: Singleton created and cached for %s", formatKey(b.key))
	return v, nil
}

func (c *Container) createTransient(b binding, st *resolveState) (any, error) {
	c.logger.Debug("Container.createTransient: Creating new transient instance for %s", formatKey(b.key))

	instance, err := b.provider.Provide(c, st)
	if err != nil {
		c.logger.Error("Container.createTransient: Failed to create transient instance for %s: %v", formatKey(b.key), err)
		return nil, err
	}

	c.lifecycle.track(instance)

	c.logger.Debug("Container.createTransient: Transient instance created for %s", formatKey(b.key))
	return instance, nil
}

func (c *Container) getOrCreateScoped(b binding, st *resolveState) (any, error) {
	if isBuiltinRequest(b.scope) {
		return c.getOrCreateRequestScoped(b, st)
	}

	if atomicScope, ok := b.scope.(AtomicScope); ok {
		return c.getOrCreateAtomicScoped(b, st, atomicScope)
	}

	if instance, ok := b.scope.Get(b.key); ok {
		c.logger.Debug("Container.getOrCreateScoped: Using cached scoped instance for %s [scope=%s]", formatKey(b.key), b.scope)
		return instance, nil
	}

	c.logger.Debug("Container.getOrCreateScoped: Creating new scoped instance for %s [scope=%s]", formatKey(b.key), b.scope)

	instance, err := b.provider.Provide(c, st)
	if err != nil {
		c.logger.Error("Container.getOrCreateScoped: Failed to create scoped instance for %s: %v", formatKey(b.key), err)
		return nil, err
	}

	b.scope.Set(b.key, instance)

	if shouldTrackInContainerLifecycle(b.scope) {
		c.lifecycle.track(instance)
	}

	c.logger.Debug("Container.getOrCreateScoped: Scoped instance created and cached for %s [scope=%s]", formatKey(b.key), b.scope)
	return instance, nil
}

func (c *Container) getOrCreateAtomicScoped(b binding, st *resolveState, atomicScope AtomicScope) (any, error) {
	instance, created, err := atomicScope.GetOrCreate(b.key, func() (any, error) {
		return b.provider.Provide(c, st)
	})
	if err != nil {
		c.logger.Error("Container.getOrCreateAtomicScoped: Failed to create scoped instance for %s [scope=%s]: %v", formatKey(b.key), b.scope, err)
		return nil, err
	}

	if created && shouldTrackInContainerLifecycle(b.scope) {
		c.lifecycle.track(instance)
	}

	if created {
		c.logger.Debug("Container.getOrCreateAtomicScoped: Scoped instance created and cached for %s [scope=%s]", formatKey(b.key), b.scope)
	} else {
		c.logger.Debug("Container.getOrCreateAtomicScoped: Using cached scoped instance for %s [scope=%s]", formatKey(b.key), b.scope)
	}
	return instance, nil
}

func (c *Container) getOrCreateRequestScoped(b binding, st *resolveState) (any, error) {
	c.scopedMu.RLock()
	if v, ok := c.scoped[b.key]; ok {
		c.scopedMu.RUnlock()
		c.logger.Debug("Container.getOrCreateRequestScoped: Using cached request instance for %s", formatKey(b.key))
		return v, nil
	}
	c.scopedMu.RUnlock()

	c.logger.Debug("Container.getOrCreateRequestScoped: Creating new request instance for %s", formatKey(b.key))
	instance, err := b.provider.Provide(c, st)
	if err != nil {
		c.logger.Error("Container.getOrCreateRequestScoped: Failed to create request instance for %s: %v", formatKey(b.key), err)
		return nil, err
	}

	c.scopedMu.Lock()
	if existing, ok := c.scoped[b.key]; ok {
		c.scopedMu.Unlock()
		c.logger.Debug("Container.getOrCreateRequestScoped: Request instance already created concurrently for %s", formatKey(b.key))
		return existing, nil
	}
	c.scoped[b.key] = instance
	c.scopedMu.Unlock()

	c.lifecycle.track(instance)
	c.logger.Debug("Container.getOrCreateRequestScoped: Request instance created and cached for %s", formatKey(b.key))
	return instance, nil
}

func shouldTrackInContainerLifecycle(scope Scope) bool {
	lifecycleAware, ok := scope.(ContainerLifecycleScope)
	if !ok {
		return false
	}
	return lifecycleAware.TrackInContainer()
}

// Invoke resolves function parameters from the container, calls fn, and returns fn's error (if any).
func (c *Container) Invoke(fn any) error {
	rv := reflect.ValueOf(fn)
	if !rv.IsValid() || rv.Kind() != reflect.Func {
		return fmt.Errorf("bender: invoke expects a function, got %T", fn)
	}

	rt := rv.Type()
	errorType := reflect.TypeOf((*error)(nil)).Elem()
	if rt.NumOut() > 1 || (rt.NumOut() == 1 && !rt.Out(0).Implements(errorType)) {
		return fmt.Errorf("bender: invoke function must return nothing or only error")
	}

	st := &resolveState{
		seen: make(map[Key]bool),
	}

	args := make([]reflect.Value, 0, rt.NumIn())
	for i := 0; i < rt.NumIn(); i++ {
		paramType := rt.In(i)

		v, err := c.resolve(keyOfType(paramType, ""), st)
		if err != nil {
			return err
		}

		arg := reflect.ValueOf(v)
		if !arg.IsValid() {
			return fmt.Errorf("bender: resolved invalid value for %s", paramType)
		}

		if arg.Type().AssignableTo(paramType) {
			args = append(args, arg)
			continue
		}

		if arg.Type().ConvertibleTo(paramType) {
			args = append(args, arg.Convert(paramType))
			continue
		}

		return fmt.Errorf("bender: resolved %s but cannot pass to invoke param %s", arg.Type(), paramType)
	}

	out := rv.Call(args)
	if len(out) == 1 && !out[0].IsNil() {
		return out[0].Interface().(error)
	}

	return nil
}
