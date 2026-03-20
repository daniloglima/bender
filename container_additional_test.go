package bender_test

import (
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/daniloglima/bender"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type cycleA struct{ b *cycleB }
type cycleB struct{ a *cycleA }

type failingDisposable struct{}

func (f *failingDisposable) Dispose() error { return errors.New("dispose failed") }

type lifecycleOptInScope struct {
	mu    sync.Mutex
	cache map[bender.Key]any
}

func newLifecycleOptInScope() *lifecycleOptInScope {
	return &lifecycleOptInScope{cache: make(map[bender.Key]any)}
}

func (s *lifecycleOptInScope) Get(key bender.Key) (any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.cache[key]
	return v, ok
}

func (s *lifecycleOptInScope) Set(key bender.Key, instance any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache[key] = instance
}

func (s *lifecycleOptInScope) String() string { return "lifecycle-opt-in" }

func (s *lifecycleOptInScope) GetOrCreate(key bender.Key, create func() (any, error)) (any, bool, error) {
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

func (s *lifecycleOptInScope) TrackInContainer() bool { return true }

func TestResolveMissingBindingError(t *testing.T) {
	container := bender.New()

	_, err := bender.Resolve[*Database](container)
	require.Error(t, err)
	var mbe bender.MissingBindingError
	assert.ErrorAs(t, err, &mbe)
}

func TestResolveCycleError(t *testing.T) {
	container := bender.New(
		bender.ModuleFunc(func(b *bender.Binder) {
			bender.Provide[*cycleA](b).Register(func(dep *cycleB) *cycleA { return &cycleA{b: dep} })
			bender.Provide[*cycleB](b).Register(func(dep *cycleA) *cycleB { return &cycleB{a: dep} })
		}),
	)

	_, err := bender.Resolve[*cycleA](container)
	require.Error(t, err)
	var ce bender.CycleError
	assert.ErrorAs(t, err, &ce)
	assert.GreaterOrEqual(t, len(ce.Cycle), 2)
}

func TestMustResolvePanicsOnMissingBinding(t *testing.T) {
	container := bender.New()
	require.Panics(t, func() {
		_ = bender.MustResolve[*Database](container)
	})
}

func TestMustResolveNamedPanicsOnMissingBinding(t *testing.T) {
	container := bender.New()
	require.Panics(t, func() {
		_ = bender.MustResolveNamed[*Database](container, "primary")
	})
}

func TestDeprecatedScopeCreators(t *testing.T) {
	container := bender.New(
		bender.ModuleFunc(func(b *bender.Binder) {
			bender.Provide[*Database](b).
				In(bender.SingletonScope()).
				Register(func() *Database { return &Database{} })
		}),
	)

	s1 := container.NewTransientScope()
	s2 := container.NewSingletonScope()

	d1 := bender.MustResolve[*Database](s1)
	d2 := bender.MustResolve[*Database](s2)
	assert.Same(t, d1, d2)
}

func TestDisposeIdempotent(t *testing.T) {
	var count atomic.Int64
	container := bender.New(
		bender.ModuleFunc(func(b *bender.Binder) {
			bender.Provide[*disposableProbe](b).
				In(bender.TransientScope()).
				Register(func() *disposableProbe { return &disposableProbe{disposedCount: &count} })
		}),
	)

	_, err := bender.Resolve[*disposableProbe](container)
	require.NoError(t, err)
	require.NoError(t, container.Dispose())
	require.NoError(t, container.Dispose())
	assert.Equal(t, int64(1), count.Load())
}

func TestDisposeReturnsFirstError(t *testing.T) {
	container := bender.New(
		bender.ModuleFunc(func(b *bender.Binder) {
			bender.Provide[*failingDisposable](b).
				In(bender.TransientScope()).
				Register(func() *failingDisposable { return &failingDisposable{} })
		}),
	)

	_, err := bender.Resolve[*failingDisposable](container)
	require.NoError(t, err)

	err = container.Dispose()
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "dispose failed"))
}

func TestCustomScopeLifecycleOptIn(t *testing.T) {
	scope := newLifecycleOptInScope()
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
	assert.Equal(t, int64(1), disposedCount.Load())
}
