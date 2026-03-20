package bender

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuiltinScopeHelpersAndDetection(t *testing.T) {
	assert.True(t, isBuiltinSingleton(SingletonScope()))
	assert.True(t, isBuiltinTransient(TransientScope()))
	assert.True(t, isBuiltinRequest(RequestScope()))
}

func TestNewRequestScopeReturnsBuiltinRequestScope(t *testing.T) {
	assert.Equal(t, RequestScope(), NewRequestScope())
}

func TestBuiltinScopeMethods(t *testing.T) {
	k := keyOfType(typeOf[int](), "")

	_, ok := TransientScope().Get(k)
	assert.False(t, ok)
	TransientScope().Set(k, 1)
	assert.NotEmpty(t, TransientScope().String())

	_, ok = SingletonScope().Get(k)
	assert.False(t, ok)
	SingletonScope().Set(k, 1)
	assert.NotEmpty(t, SingletonScope().String())

	_, ok = RequestScope().Get(k)
	assert.False(t, ok)
	RequestScope().Set(k, 1)
	assert.NotEmpty(t, RequestScope().String())

	lifecycleAware, ok := RequestScope().(ContainerLifecycleScope)
	require.True(t, ok)
	assert.True(t, lifecycleAware.TrackInContainer())
}
