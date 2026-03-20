package bender_test

import (
	"testing"

	"github.com/daniloglima/bender"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstanceBindsSingletonByDefault(t *testing.T) {
	type Config struct{ Value string }

	cfg := &Config{Value: "ok"}
	container := bender.New(
		bender.ModuleFunc(func(b *bender.Binder) {
			bender.Instance[*Config](b, cfg)
		}),
	)

	c1 := bender.MustResolve[*Config](container)
	c2 := bender.MustResolve[*Config](container)
	assert.Same(t, c1, c2)
	assert.Same(t, cfg, c1)
}

func TestOriginAndNamedBinding(t *testing.T) {
	type Service struct{ Name string }

	container := bender.New(
		bender.ModuleFunc(func(b *bender.Binder) {
			bender.Provide[*Service](b).
				Named("primary").
				Origin("tests/module.go:10").
				Register(func() *Service { return &Service{Name: "primary"} })
		}),
	)

	svc, err := bender.ResolveNamed[*Service](container, "primary")
	require.NoError(t, err)
	assert.Equal(t, "primary", svc.Name)
}

func TestDuplicateBindingPanics(t *testing.T) {
	require.Panics(t, func() {
		b := bender.NewBinder()
		bender.Provide[*Database](b).Register(func() *Database { return &Database{} })
		bender.Provide[*Database](b).Register(func() *Database { return &Database{} })
	})
}

func TestRegisterPanicsOnInvalidProvider(t *testing.T) {
	require.Panics(t, func() {
		b := bender.NewBinder()
		bender.Provide[*Database](b).Register(10)
	})
}

func TestInstallSkipsNilModule(t *testing.T) {
	container := bender.New(
		nil,
		bender.ModuleFunc(func(b *bender.Binder) {
			bender.Provide[*Database](b).Register(func() *Database { return &Database{} })
		}),
	)

	_, err := bender.Resolve[*Database](container)
	require.NoError(t, err)
}
