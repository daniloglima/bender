package bender_test

import (
	"testing"

	"github.com/daniloglima/bender"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderOf(t *testing.T) {
	container := bender.New(
		bender.ModuleFunc(func(b *bender.Binder) {
			bender.Provide[*Database](b).
				In(bender.SingletonScope()).
				Register(func() *Database { return &Database{ConnectionString: "dsn"} })
		}),
	)

	provider, err := bender.ProviderOf[*Database](container)
	require.NoError(t, err)

	db, err := provider()
	require.NoError(t, err)
	assert.Equal(t, "dsn", db.ConnectionString)
}

func TestProviderNamedFunc(t *testing.T) {
	type Client struct{ Name string }

	container := bender.New(
		bender.ModuleFunc(func(b *bender.Binder) {
			bender.Provide[*Client](b).Named("read").Register(func() *Client { return &Client{Name: "read"} })
			bender.Provide[*Client](b).Named("write").Register(func() *Client { return &Client{Name: "write"} })
		}),
	)

	provider := bender.ProviderNamedFunc[*Client](container)

	read, err := provider("read")
	require.NoError(t, err)
	assert.Equal(t, "read", read.Name)
}
