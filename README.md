# Bender - Dependency Injection Container for Go

Bender is a lightweight dependency injection container for Go with:

- type-safe resolution through generics
- pluggable scopes
- lazy providers
- lifecycle disposal
- cycle detection
- configurable logging

## Install

```bash
go get github.com/daniloglima/bender
```

## Quick Start

```go
package main

import (
    "fmt"

    "github.com/daniloglima/bender"
)

type Config struct {
    DSN string
}

type Repository struct {
    cfg *Config
}

func main() {
    container := bender.New(
        bender.ModuleFunc(func(b *bender.Binder) {
            bender.Provide[*Config](b).
                In(bender.ScopeSingleton).
                Register(func() *Config {
                    return &Config{DSN: "postgres://localhost:5432/app"}
                })

            bender.Provide[*Repository](b).
                In(bender.ScopeTransient).
                Register(func(cfg *Config) *Repository {
                    return &Repository{cfg: cfg}
                })
        }),
    )

    repo := bender.MustResolve[*Repository](container)
    fmt.Println(repo.cfg.DSN)
}
```

## Main Patterns

### Standard `main.go` (single module inline)

Good for small apps and prototypes.

```go
package main

import "github.com/daniloglima/bender"

func main() {
    container := bender.New(
        bender.ModuleFunc(func(b *bender.Binder) {
            bender.Provide[*Config](b).
                In(bender.SingletonScope()).
                Register(NewConfig)

            bender.Provide[*App](b).
                In(bender.SingletonScope()).
                Register(NewApp)
        }),
    )

    app := bender.MustResolve[*App](container)
    app.Run()
}
```

### Modular `main.go` (`di/modules` + `container`)

Recommended for medium/large services.

`di/modules/config_module.go`:

```go
package modules

import "github.com/daniloglima/bender"

func ConfigModule() bender.Module {
    return bender.ModuleFunc(func(b *bender.Binder) {
        bender.Provide[*Config](b).
            In(bender.SingletonScope()).
            Register(NewConfig)
    })
}
```

`di/modules/http_module.go`:

```go
package modules

import "github.com/daniloglima/bender"

func HTTPModule() bender.Module {
    return bender.ModuleFunc(func(b *bender.Binder) {
        bender.Provide[*HTTPServer](b).
            In(bender.SingletonScope()).
            Register(NewHTTPServer)
    })
}
```

`di/container.go`:

```go
package di

import (
    "github.com/daniloglima/bender"
    "github.com/your-org/yourapp/di/modules"
)

func Build() *bender.Container {
    return bender.New(
        modules.ConfigModule(),
        modules.HTTPModule(),
    )
}
```

`cmd/api/main.go`:

```go
package main

import "github.com/your-org/yourapp/di"

func main() {
    container := di.Build()
    server := bender.MustResolve[*HTTPServer](container)
    server.Start()
}
```

### Which one to choose?

- Use standard `main.go` when wiring is small and unlikely to grow.
- Use modular container build when multiple domains/teams need clear ownership.
- In modular mode, keep each `di/modules/*_module.go` focused on one domain or infrastructure concern.

## DI Tool Comparison (Bender vs Wire vs Dig vs Fx)

Inspired by community comparisons such as:
- https://dev.to/rezende79/dependency-injection-in-go-comparing-wire-dig-fx-more-3nkj

| Tool | DI Model | Reflection | Compile-Time Graph Safety | Lifecycle Support | Typical Fit |
| --- | --- | --- | --- | --- | --- |
| **Bender** | Runtime container with typed APIs | Minimal internal reflection (provider invocation) | Partial (type checks + runtime graph checks) | Built-in disposable tracking + scoped disposal | Teams wanting a pragmatic DI container with scopes and low framework lock-in |
| **Wire** | Compile-time code generation | No | Strong (compile-time) | Not built-in as framework lifecycle | Projects prioritizing explicit generated wiring and compile-time guarantees |
| **Dig** | Runtime container | Yes | Runtime only | Basic (container-level) | Flexible runtime wiring and framework internals |
| **Fx** | Runtime framework on top of Dig | Yes | Runtime only | Strong (`OnStart` / `OnStop`, app framework lifecycle) | Large services needing opinionated app lifecycle and modules |

### Practical trade-offs

1. **Bender vs Wire**
   - Bender reduces generator workflow overhead and supports pluggable scopes directly.
   - Wire offers stronger compile-time graph validation via generated code.
2. **Bender vs Dig**
   - Both support runtime wiring.
   - Bender focuses on typed resolution helpers, pluggable scopes, and explicit scope disposal patterns.
3. **Bender vs Fx**
   - Bender is a DI container (lighter, less opinionated).
   - Fx is an application framework (more batteries included, higher adoption overhead).

### Current status note

As of **August 25, 2025**, `google/wire` was archived and is read-only.  
If long-term active maintenance is a hard requirement, consider this in your tool selection.

## Resolution APIs

```go
value, err := bender.Resolve[*MyType](container)
namedValue, err := bender.ResolveNamed[*MyType](container, "primary")

must := bender.MustResolve[*MyType](container)
mustNamed := bender.MustResolveNamed[*MyType](container, "primary")
```

## Bindings

### Default Binding

```go
bender.Provide[*Service](b).
    Register(func(dep *Dependency) *Service {
        return &Service{dep: dep}
    })
```

### Singleton

```go
bender.Provide[*Database](b).
    In(bender.ScopeSingleton).
    Register(func() *Database {
        return &Database{}
    })
```

### Named Bindings

```go
bender.Provide[*Client](b).
    Named("read").
    Register(func() *Client { return NewReadClient() })

bender.Provide[*Client](b).
    Named("write").
    Register(func() *Client { return NewWriteClient() })

readClient, _ := bender.ResolveNamed[*Client](container, "read")
_ = readClient
```

## Scopes

Bender includes:

- `ScopeSingleton`: one instance per root container
- `ScopeTransient`: new instance per resolve
- `ScopeRequest`: one instance per created scope (`CreateScope`)

Prefer helper functions `SingletonScope()`, `TransientScope()`, and `RequestScope()` in new code.

```go
bender.Provide[*RequestContext](b).
    In(bender.ScopeRequest).
    Register(func() *RequestContext {
        return &RequestContext{}
    })

scope := root.CreateScope()
defer scope.Dispose()

ctx1, _ := bender.Resolve[*RequestContext](scope)
ctx2, _ := bender.Resolve[*RequestContext](scope)
// ctx1 == ctx2 inside this scope
```

### Custom Scopes

You can implement custom scopes through `Scope`.

For concurrent-safe, single-creation semantics under load, implement `AtomicScope`.

By default, custom scope instances are not tracked by container lifecycle disposal.
If you want container-managed disposal for custom scope instances, implement
`ContainerLifecycleScope` and return `true` from `TrackInContainer()`.

#### How to add a custom scope

1. Create a type that implements `Scope`.
2. If you need atomic `get-or-create` behavior under concurrency, also implement `AtomicScope`.
3. Decide lifecycle behavior:
   - default: custom scope instances are **not** tracked by container disposal
   - opt-in: implement `ContainerLifecycleScope` and return `true`
4. Bind dependencies using `.In(myScope)`.

Example:

```go
package main

import (
    "sync"

    "github.com/daniloglima/bender"
)

// TenantScope caches one instance per key in this scope object.
type TenantScope struct {
    mu    sync.Mutex
    cache map[bender.Key]any
}

func NewTenantScope() *TenantScope {
    return &TenantScope{cache: make(map[bender.Key]any)}
}

func (s *TenantScope) String() string { return "tenant" }

func (s *TenantScope) Get(key bender.Key) (any, bool) {
    s.mu.Lock()
    defer s.mu.Unlock()
    v, ok := s.cache[key]
    return v, ok
}

func (s *TenantScope) Set(key bender.Key, instance any) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.cache[key] = instance
}

// Optional: atomic creation path (recommended for concurrent services).
func (s *TenantScope) GetOrCreate(key bender.Key, create func() (any, error)) (any, bool, error) {
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

// Optional: let container Dispose() track instances created in this scope.
func (s *TenantScope) TrackInContainer() bool { return true }

func register(b *bender.Binder) {
    tenantScope := NewTenantScope()

    bender.Provide[*TenantService](b).
        In(tenantScope).
        Register(NewTenantService)
}
```

## Lazy Providers

When a singleton depends on short-lived objects, inject a provider function instead of a concrete instance.

```go
type Server struct {
    serviceProvider bender.LazyProvider[*Service]
}

bender.Provide[bender.LazyProvider[*Service]](b).
    In(bender.ScopeSingleton).
    Register(func(c *bender.Container) bender.LazyProvider[*Service] {
        return bender.ProviderFunc[*Service](c)
    })

bender.Provide[*Server](b).
    In(bender.ScopeSingleton).
    Register(func(p bender.LazyProvider[*Service]) *Server {
        return &Server{serviceProvider: p}
    })
```

## Lifecycle Management

If an instance implements `Disposable`, Bender tracks it and calls `Dispose()` when the scope/container is disposed.

```go
type Resource struct{}

func (r *Resource) Dispose() error {
    return nil
}

scope := root.CreateScope()
defer scope.Dispose()
```

## Invoke

`Invoke` resolves function parameters from the container and executes the function.

```go
err := container.Invoke(func(repo *Repository) error {
    // use repo
    return nil
})
```

## Logging

Set logging level by environment variable:

```bash
export BENDER_LOG_LEVEL=debug
```

Supported values:

- `none`
- `error`
- `info`
- `debug`

Programmatic configuration:

```go
container := bender.NewWithOptions(
    []bender.ContainerOption{bender.WithLogLevel(bender.LogLevelInfo)},
    MyModule(),
)
```

Detailed policy (levels, message style, and noise control):

- [LOGGING_POLICY.md](./LOGGING_POLICY.md)

## Error Types

- `MissingBindingError`
- `CycleError`

## Best Practices

1. Use `ScopeSingleton` for stateless shared services.
2. Use `ScopeTransient` for short-lived operations.
3. Use `ScopeRequest` with `CreateScope()` for per-request state.
4. Use `LazyProvider[T]` when singleton components need transient dependencies.
5. Always call `Dispose()` on created scopes.
6. Enable `debug` logging only for troubleshooting.

## Stability

Bender is ready for early OSS usage. For production adoption, pin versions and review release notes.
