package bender

// LazyProvider provides lazy resolution of dependencies.
// This allows injecting a factory function instead of a concrete instance.
type LazyProvider[T any] func() (T, error)
type LazyNamedProvider[T any] func(name string) (T, error)

// ProviderFunc is a helper to create LazyProvider bindings.
func ProviderFunc[T any](c *Container) LazyProvider[T] {
	return func() (T, error) {
		return Resolve[T](c)
	}
}

// ProviderNamedFunc is a helper to create LazyNamedProvider bindings.
func ProviderNamedFunc[T any](c *Container) LazyNamedProvider[T] {
	return func(name string) (T, error) {
		return ResolveNamed[T](c, name)
	}
}

// ProviderOf resolves a LazyProvider for type T from the container.
// This is useful when you want to inject a factory instead of an instance.
func ProviderOf[T any](c *Container) (LazyProvider[T], error) {
	return ProviderFunc[T](c), nil
}
