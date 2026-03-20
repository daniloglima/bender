package bender_test

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/daniloglima/bender"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Example service hierarchy
type Database struct {
	ConnectionString string
	Queries          int
}

func TestInvoke(t *testing.T) {
	container := bender.New(
		bender.ModuleFunc(func(b *bender.Binder) {
			bender.Provide[*Database](b).
				In(bender.ScopeSingleton).
				Register(func() *Database {
					return &Database{ConnectionString: "postgres://invoke"}
				})
		}),
	)

	called := false
	err := container.Invoke(func(db *Database) error {
		called = true
		assert.Equal(t, "postgres://invoke", db.ConnectionString)
		return nil
	})
	require.NoError(t, err)
	assert.True(t, called)
}

func TestInvokeReturnsFunctionError(t *testing.T) {
	container := bender.New()
	expectedErr := errors.New("boom")

	err := container.Invoke(func() error {
		return expectedErr
	})
	assert.ErrorIs(t, err, expectedErr)
}

func TestInvokeRejectsInvalidSignature(t *testing.T) {
	container := bender.New()

	err := container.Invoke(func() int {
		return 1
	})
	require.Error(t, err)
}

func TestResolveNamed(t *testing.T) {
	type Handler struct {
		Name string
	}

	container := bender.New(
		bender.ModuleFunc(func(b *bender.Binder) {
			bender.Provide[*Handler](b).
				Named("auth").
				Register(func() *Handler { return &Handler{Name: "auth"} })

			bender.Provide[*Handler](b).
				Named("logging").
				Register(func() *Handler { return &Handler{Name: "logging"} })
		}),
	)

	auth, err := bender.ResolveNamed[*Handler](container, "auth")
	require.NoError(t, err)
	assert.Equal(t, "auth", auth.Name)

	logging := bender.MustResolveNamed[*Handler](container, "logging")
	assert.Equal(t, "logging", logging.Name)
}

func TestRequestScopeIsIsolatedPerContainerScope(t *testing.T) {
	type RequestContext struct {
		ID int
	}

	nextID := 0
	root := bender.New(
		bender.ModuleFunc(func(b *bender.Binder) {
			bender.Provide[*RequestContext](b).
				In(bender.ScopeRequest).
				Register(func() *RequestContext {
					nextID++
					return &RequestContext{ID: nextID}
				})
		}),
	)

	scope1 := root.CreateScope()
	scope2 := root.CreateScope()

	s1a, err := bender.Resolve[*RequestContext](scope1)
	require.NoError(t, err)
	s1b, err := bender.Resolve[*RequestContext](scope1)
	require.NoError(t, err)
	assert.Same(t, s1a, s1b)

	s2, err := bender.Resolve[*RequestContext](scope2)
	require.NoError(t, err)
	assert.NotSame(t, s1a, s2)
}

type disposableProbe struct {
	disposedCount *atomic.Int64
}

func (d *disposableProbe) Dispose() error {
	d.disposedCount.Add(1)
	return nil
}

func TestLifecycleConcurrentTrackAndDispose(t *testing.T) {
	var disposedCount atomic.Int64
	container := bender.New(
		bender.ModuleFunc(func(b *bender.Binder) {
			bender.Provide[*disposableProbe](b).
				In(bender.ScopeTransient).
				Register(func() *disposableProbe {
					return &disposableProbe{disposedCount: &disposedCount}
				})
		}),
	)

	const workers = 64
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			if _, err := bender.Resolve[*disposableProbe](container); err != nil {
				t.Errorf("resolve failed: %v", err)
			}
		}()
	}
	wg.Wait()

	if err := container.Dispose(); err != nil {
		require.NoError(t, err)
	}

	assert.Equal(t, int64(workers), disposedCount.Load())
}

type atomicTestScope struct {
	mu    sync.Mutex
	cache map[bender.Key]any
}

func newAtomicTestScope() *atomicTestScope {
	return &atomicTestScope{
		cache: make(map[bender.Key]any),
	}
}

func (s *atomicTestScope) Get(key bender.Key) (any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.cache[key]
	return v, ok
}

func (s *atomicTestScope) Set(key bender.Key, instance any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache[key] = instance
}

func (s *atomicTestScope) String() string {
	return "atomic-test"
}

func (s *atomicTestScope) GetOrCreate(key bender.Key, create func() (any, error)) (any, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if v, ok := s.cache[key]; ok {
		return v, false, nil
	}

	v, err := create()
	if err != nil {
		return nil, false, err
	}

	s.cache[key] = v
	return v, true, nil
}

func TestAtomicScopeAvoidsDuplicateConstruction(t *testing.T) {
	type Resource struct {
		ID int64
	}

	scope := newAtomicTestScope()
	var created atomic.Int64
	container := bender.New(
		bender.ModuleFunc(func(b *bender.Binder) {
			bender.Provide[*Resource](b).
				In(scope).
				Register(func() *Resource {
					return &Resource{ID: created.Add(1)}
				})
		}),
	)

	const workers = 64
	results := make([]*Resource, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		i := i
		go func() {
			defer wg.Done()
			v, err := bender.Resolve[*Resource](container)
			if err != nil {
				t.Errorf("resolve failed: %v", err)
				return
			}
			results[i] = v
		}()
	}
	wg.Wait()

	assert.Equal(t, int64(1), created.Load())
	for i := 1; i < workers; i++ {
		assert.Same(t, results[0], results[i])
	}
}

func TestCustomScopeDoesNotTrackLifecycleByDefault(t *testing.T) {
	scope := newAtomicTestScope()
	var disposedCount atomic.Int64

	container := bender.New(
		bender.ModuleFunc(func(b *bender.Binder) {
			bender.Provide[*disposableProbe](b).
				In(scope).
				Register(func() *disposableProbe {
					return &disposableProbe{disposedCount: &disposedCount}
				})
		}),
	)

	_, err := bender.Resolve[*disposableProbe](container)
	require.NoError(t, err)

	require.NoError(t, container.Dispose())

	assert.Equal(t, int64(0), disposedCount.Load())
}

func TestBindingBuilderInPanicsOnNilScope(t *testing.T) {
	require.Panics(t, func() {
		b := bender.NewBinder()
		bender.Provide[*Database](b).In(nil)
	})
}

func TestProvideDefaultScopeIgnoresExportedVarReassignment(t *testing.T) {
	originalTransient := bender.ScopeTransient
	defer func() { bender.ScopeTransient = originalTransient }()
	bender.ScopeTransient = bender.SingletonScope()

	container := bender.New(
		bender.ModuleFunc(func(b *bender.Binder) {
			bender.Provide[*Database](b).Register(func() *Database { return &Database{} })
		}),
	)

	db1 := bender.MustResolve[*Database](container)
	db2 := bender.MustResolve[*Database](container)
	assert.NotSame(t, db1, db2)
}

func TestBuiltinScopeHelpersRemainStableAfterExportedVarReassignment(t *testing.T) {
	origSingleton := bender.ScopeSingleton
	origTransient := bender.ScopeTransient
	origRequest := bender.ScopeRequest
	defer func() {
		bender.ScopeSingleton = origSingleton
		bender.ScopeTransient = origTransient
		bender.ScopeRequest = origRequest
	}()

	bender.ScopeSingleton = bender.TransientScope()
	bender.ScopeTransient = bender.SingletonScope()
	bender.ScopeRequest = bender.TransientScope()

	container := bender.New(
		bender.ModuleFunc(func(b *bender.Binder) {
			bender.Provide[*Database](b).
				In(bender.SingletonScope()).
				Register(func() *Database { return &Database{} })
		}),
	)

	db1 := bender.MustResolve[*Database](container)
	db2 := bender.MustResolve[*Database](container)
	assert.Same(t, db1, db2)
}

func TestSingletonInitializedOnceUnderConcurrency(t *testing.T) {
	type ExpensiveService struct {
		ID int64
	}

	var initCount atomic.Int64
	container := bender.New(
		bender.ModuleFunc(func(b *bender.Binder) {
			bender.Provide[*ExpensiveService](b).
				In(bender.ScopeSingleton).
				Register(func() *ExpensiveService {
					return &ExpensiveService{ID: initCount.Add(1)}
				})
		}),
	)

	const workers = 128
	results := make([]*ExpensiveService, workers)
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		i := i
		go func() {
			defer wg.Done()
			svc, err := bender.Resolve[*ExpensiveService](container)
			if err != nil {
				t.Errorf("resolve failed: %v", err)
				return
			}
			results[i] = svc
		}()
	}
	wg.Wait()

	assert.Equal(t, int64(1), initCount.Load())
	for i := 1; i < workers; i++ {
		assert.Same(t, results[0], results[i])
	}
}

func (db *Database) Query() {
	db.Queries++
}

type UserRepository struct {
	DB *Database
}

func NewUserRepository(db *Database) *UserRepository {
	return &UserRepository{DB: db}
}

type UserService struct {
	Repo *UserRepository
}

func NewUserService(repo *UserRepository) *UserService {
	return &UserService{Repo: repo}
}

// TestSingletonScope verifies that singletons are created once and cached
func TestSingletonScope(t *testing.T) {
	container := bender.NewWithOptions(
		[]bender.ContainerOption{bender.WithDebug()},
		bender.ModuleFunc(func(b *bender.Binder) {
			bender.Provide[*Database](b).
				In(bender.ScopeSingleton).
				Register(func() *Database {
					return &Database{ConnectionString: "postgres://localhost"}
				})
		}),
	)

	// Resolve twice
	db1, err := bender.Resolve[*Database](container)
	require.NoError(t, err)

	db2, err := bender.Resolve[*Database](container)
	require.NoError(t, err)

	assert.Same(t, db1, db2)
}

// TestTransientScope verifies that transients are created fresh each time
func TestTransientScope(t *testing.T) {
	container := bender.NewWithOptions(
		[]bender.ContainerOption{bender.WithDebug()},
		bender.ModuleFunc(func(b *bender.Binder) {
			bender.Provide[*UserService](b).
				In(bender.ScopeTransient).
				Register(func() *UserService {
					return &UserService{}
				})
		}),
	)

	// Resolve twice
	svc1, err := bender.Resolve[*UserService](container)
	require.NoError(t, err)

	svc2, err := bender.Resolve[*UserService](container)
	require.NoError(t, err)

	assert.NotSame(t, svc1, svc2)
}

// TestLazyProvider demonstrates the correct way to inject transients into singletons
func TestLazyProvider(t *testing.T) {
	type HTTPServer struct {
		serviceProvider bender.LazyProvider[*UserService]
		requestCount    int
	}

	container := bender.NewWithOptions(
		[]bender.ContainerOption{bender.WithDebug()},
		bender.ModuleFunc(func(b *bender.Binder) {
			// Transient service
			bender.Provide[*UserService](b).
				In(bender.ScopeTransient).
				Register(func() *UserService {
					return &UserService{}
				})

			// Lazy provider for the service (singleton)
			bender.Provide[bender.LazyProvider[*UserService]](b).
				In(bender.ScopeSingleton).
				Register(func(c *bender.Container) bender.LazyProvider[*UserService] {
					return bender.ProviderFunc[*UserService](c)
				})

			// Singleton server receives the provider
			bender.Provide[*HTTPServer](b).
				In(bender.ScopeSingleton).
				Register(func(provider bender.LazyProvider[*UserService]) *HTTPServer {
					return &HTTPServer{serviceProvider: provider}
				})
		}),
	)

	// Get the singleton server
	server, err := bender.Resolve[*HTTPServer](container)
	require.NoError(t, err)

	// Simulate 3 requests
	var services []*UserService
	for i := 0; i < 3; i++ {
		service, err := server.serviceProvider()
		require.NoError(t, err)
		services = append(services, service)
		server.requestCount++
	}

	// All services should be different instances
	for i := 0; i < len(services); i++ {
		for j := i + 1; j < len(services); j++ {
			assert.NotSame(t, services[i], services[j], "Expected different instances for request %d and %d", i+1, j+1)
		}
	}

	assert.Equal(t, 3, server.requestCount)
}

// TestDependencyInjection verifies that dependencies are correctly injected
func TestDependencyInjection(t *testing.T) {
	container := bender.NewWithOptions(
		nil,
		bender.ModuleFunc(func(b *bender.Binder) {
			bender.Provide[*Database](b).
				In(bender.ScopeSingleton).
				Register(func() *Database {
					return &Database{ConnectionString: "postgres://localhost"}
				})

			bender.Provide[*UserRepository](b).
				In(bender.ScopeSingleton).
				Register(NewUserRepository)

			bender.Provide[*UserService](b).
				In(bender.ScopeTransient).
				Register(NewUserService)
		}),
	)

	// Resolve service
	service, err := bender.Resolve[*UserService](container)
	require.NoError(t, err)

	require.NotNil(t, service.Repo)
	require.NotNil(t, service.Repo.DB)
	assert.Equal(t, "postgres://localhost", service.Repo.DB.ConnectionString)
}

// TestScopedContainer verifies scoped container behavior
func TestScopedContainer(t *testing.T) {
	type RequestContext struct {
		RequestID string
	}

	rootContainer := bender.NewWithOptions(
		nil,
		bender.ModuleFunc(func(b *bender.Binder) {
			bender.Provide[*RequestContext](b).
				In(bender.ScopeTransient).
				Register(func() *RequestContext {
					return &RequestContext{RequestID: "new-request"}
				})
		}),
	)

	// Create scoped containers for different requests
	scope1 := rootContainer.CreateScope()
	ctx1, _ := bender.Resolve[*RequestContext](scope1)

	scope2 := rootContainer.CreateScope()
	ctx2, _ := bender.Resolve[*RequestContext](scope2)

	assert.NotSame(t, ctx1, ctx2)
}

// BenchmarkSingletonResolution benchmarks singleton resolution
func BenchmarkSingletonResolution(b *testing.B) {
	container := bender.NewWithOptions(
		nil,
		bender.ModuleFunc(func(binder *bender.Binder) {
			bender.Provide[*Database](binder).
				In(bender.ScopeSingleton).
				Register(func() *Database {
					return &Database{ConnectionString: "postgres://localhost"}
				})
		}),
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = bender.Resolve[*Database](container)
	}
}

// BenchmarkTransientResolution benchmarks transient resolution
func BenchmarkTransientResolution(b *testing.B) {
	container := bender.NewWithOptions(
		nil,
		bender.ModuleFunc(func(binder *bender.Binder) {
			bender.Provide[*UserService](binder).
				In(bender.ScopeTransient).
				Register(func() *UserService {
					return &UserService{}
				})
		}),
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = bender.Resolve[*UserService](container)
	}
}

// BenchmarkLazyProviderResolution benchmarks lazy provider resolution
func BenchmarkLazyProviderResolution(b *testing.B) {
	container := bender.NewWithOptions(
		nil,
		bender.ModuleFunc(func(binder *bender.Binder) {
			bender.Provide[*UserService](binder).
				In(bender.ScopeTransient).
				Register(func() *UserService {
					return &UserService{}
				})

			bender.Provide[bender.LazyProvider[*UserService]](binder).
				In(bender.ScopeSingleton).
				Register(func(c *bender.Container) bender.LazyProvider[*UserService] {
					return bender.ProviderFunc[*UserService](c)
				})
		}),
	)

	provider, _ := bender.Resolve[bender.LazyProvider[*UserService]](container)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = provider()
	}
}
