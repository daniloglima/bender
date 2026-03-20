package bender

var (
	builtinScopeTransient Scope = &transientScope{}
	builtinScopeSingleton Scope = &singletonScope{}
	builtinScopeRequest   Scope = &requestScope{}
)

var (
	// Deprecated: prefer TransientScope() to avoid accidental reassignment.
	ScopeTransient Scope = builtinScopeTransient
	// Deprecated: prefer SingletonScope() to avoid accidental reassignment.
	ScopeSingleton Scope = builtinScopeSingleton
	// Deprecated: prefer RequestScope() to avoid accidental reassignment.
	ScopeRequest Scope = builtinScopeRequest
)

func TransientScope() Scope {
	return builtinScopeTransient
}

func SingletonScope() Scope {
	return builtinScopeSingleton
}

func RequestScope() Scope {
	return builtinScopeRequest
}

// transientScope never caches instances.
type transientScope struct{}

func (s *transientScope) Get(_ Key) (any, bool) {
	return nil, false
}

func (s *transientScope) Set(_ Key, _ any) {}

func (s *transientScope) String() string {
	return "transient"
}

// singletonScope caches instances globally (at root container).
type singletonScope struct{}

func (s *singletonScope) Get(_ Key) (any, bool) {
	return nil, false
}

func (s *singletonScope) Set(_ Key, _ any) {}

func (s *singletonScope) String() string {
	return "singleton"
}

// requestScope caches per container scope through Container.scoped.
type requestScope struct{}

func NewRequestScope() Scope {
	return RequestScope()
}

func (s *requestScope) Get(_ Key) (any, bool) {
	return nil, false
}

func (s *requestScope) Set(_ Key, _ any) {}

func (s *requestScope) String() string {
	return "request"
}

func (s *requestScope) TrackInContainer() bool {
	return true
}

func isBuiltinTransient(scope Scope) bool {
	_, ok := scope.(*transientScope)
	return ok
}

func isBuiltinSingleton(scope Scope) bool {
	_, ok := scope.(*singletonScope)
	return ok
}

func isBuiltinRequest(scope Scope) bool {
	_, ok := scope.(*requestScope)
	return ok
}
