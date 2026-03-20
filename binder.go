package bender

import (
	"fmt"
	"reflect"
	"sync"
)

type binding struct {
	key      Key
	scope    Scope
	provider Provider
	origin   string // for debug (optional)
}

type Binder struct {
	mu       sync.Mutex
	bindings map[Key]binding
	logger   Logger
}

func NewBinder() *Binder {
	return &Binder{
		bindings: make(map[Key]binding),
		logger:   NoopLogger{},
	}
}

func NewBinderWithLogger(logger Logger) *Binder {
	return &Binder{
		bindings: make(map[Key]binding),
		logger:   logger,
	}
}

type BindingBuilder[T any] struct {
	binder *Binder
	key    Key
	scope  Scope
	origin string
}

func typeOf[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}

// Provide creates a builder for type T (default scope: transient).
// NOTE: Go does not support generic methods, so this is a generic function.
func Provide[T any](b *Binder) *BindingBuilder[T] {
	k := keyOfType(typeOf[T](), "")
	return &BindingBuilder[T]{
		binder: b,
		key:    k,
		scope:  TransientScope(),
	}
}

// Instance binds a concrete instance as singleton by default.
// NOTE: Go does not support generic methods, so this is a generic function.
func Instance[T any](b *Binder, v T) *BindingBuilder[T] {
	k := keyOfType(typeOf[T](), "")
	b.logger.Debug("Binder.Instance: Binding instance for %s", formatKey(k))
	bb := &BindingBuilder[T]{
		binder: b,
		key:    k,
		scope:  SingletonScope(),
	}
	bb.commit(instanceProvider{v: v})
	return bb
}

func (bb *BindingBuilder[T]) In(scope Scope) *BindingBuilder[T] {
	if scope == nil {
		panic("bender: scope cannot be nil")
	}
	bb.scope = scope
	return bb
}

func (bb *BindingBuilder[T]) Named(name string) *BindingBuilder[T] {
	bb.key.name = name
	return bb
}

func (bb *BindingBuilder[T]) Origin(origin string) *BindingBuilder[T] {
	bb.origin = origin
	return bb
}

// Register registers a provider function for T.
// fn must be func(...) T or func(...) (T, error).
func (bb *BindingBuilder[T]) Register(fn any) *BindingBuilder[T] {
	t := typeOf[T]()
	bb.binder.logger.Debug("BindingBuilder.Register: Registering provider for %s", formatKey(bb.key))
	p, err := newFuncProvider(fn, t)
	if err != nil {
		bb.binder.logger.Error("BindingBuilder.Register: Invalid provider for %s: %v", t, err)
		panic(fmt.Errorf("bender: invalid provider for %s: %w", t, err))
	}
	bb.commit(p)
	return bb
}

func (bb *BindingBuilder[T]) commit(p Provider) {
	if p == nil {
		bb.binder.logger.Error("BindingBuilder.commit: Nil provider for %s", formatKey(bb.key))
		panic("bender: nil provider")
	}

	bb.binder.mu.Lock()
	defer bb.binder.mu.Unlock()

	bd := binding{
		key:      bb.key,
		scope:    bb.scope,
		provider: p,
		origin:   bb.origin,
	}

	if _, exists := bb.binder.bindings[bb.key]; exists {
		bb.binder.logger.Error("BindingBuilder.commit: Duplicate binding for %s", formatKey(bb.key))
		panic(fmt.Errorf("bender: duplicate binding for type %s (name=%q)", bb.key.t, bb.key.name))
	}

	bb.binder.bindings[bb.key] = bd
	bb.binder.logger.Info("BindingBuilder.commit: Binding registered for %s [scope=%s]", formatKey(bb.key), bb.scope)
}

func (b *Binder) Install(mods ...Module) {
	b.logger.Info("Binder.Install: Installing %d module(s)", len(mods))
	for _, m := range mods {
		if m == nil {
			b.logger.Debug("Binder.Install: Skipping nil module")
			continue
		}
		b.logger.Info("Binder.Install: Installing module %T", m)
		m.Configure(b)
		b.logger.Debug("Binder.Install: Module %T configured successfully", m)
	}
	b.logger.Info("Binder.Install: All modules installed successfully")
}
