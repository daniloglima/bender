package bender

// Scope defines the lifecycle strategy for a dependency.
type Scope interface {
	// Get retrieves an instance from this scope's cache, if applicable.
	// Returns (instance, true) if found, (nil, false) otherwise.
	Get(key Key) (any, bool)

	// Set stores an instance in this scope's cache, if applicable.
	Set(key Key, instance any)

	// String returns the name of this scope for debugging.
	String() string
}

// AtomicScope can provide atomic per-key creation semantics.
// When implemented, Bender calls GetOrCreate to avoid duplicate construction
// under concurrent resolution.
type AtomicScope interface {
	GetOrCreate(key Key, create func() (any, error)) (instance any, created bool, err error)
}

// ContainerLifecycleScope allows a custom scope to opt in to container-managed
// lifecycle tracking (Disposable handling). By default, custom scopes are not
// tracked by containers.
type ContainerLifecycleScope interface {
	TrackInContainer() bool
}

// ScopeKind is a backward-compatible type alias for predefined scopes.
type ScopeKind = Scope
