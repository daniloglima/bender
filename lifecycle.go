package bender

import "sync"

// Disposable represents an object that needs cleanup.
type Disposable interface {
	Dispose() error
}

// DisposableFunc is a function adapter for Disposable.
type DisposableFunc func() error

func (f DisposableFunc) Dispose() error {
	return f()
}

// lifecycleManager manages disposable instances.
type lifecycleManager struct {
	mu          sync.Mutex
	disposables []Disposable
	disposed    bool
}

func newLifecycleManager() *lifecycleManager {
	return &lifecycleManager{
		disposables: make([]Disposable, 0),
	}
}

func (lm *lifecycleManager) track(instance any) {
	if d, ok := instance.(Disposable); ok {
		lm.mu.Lock()
		defer lm.mu.Unlock()
		if lm.disposed {
			return
		}
		lm.disposables = append(lm.disposables, d)
	}
}

func (lm *lifecycleManager) dispose() error {
	lm.mu.Lock()
	if lm.disposed {
		lm.mu.Unlock()
		return nil
	}
	lm.disposed = true
	disposables := append([]Disposable(nil), lm.disposables...)
	lm.disposables = nil
	lm.mu.Unlock()

	// Dispose in reverse order (LIFO)
	for i := len(disposables) - 1; i >= 0; i-- {
		if err := disposables[i].Dispose(); err != nil {
			return err
		}
	}
	return nil
}
