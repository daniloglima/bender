package bender

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFuncProviderValidation(t *testing.T) {
	intType := reflect.TypeOf(0)

	tests := []struct {
		name    string
		fn      any
		outType reflect.Type
		wantErr string
	}{
		{name: "non function", fn: 10, outType: intType, wantErr: "provider must be a function"},
		{name: "no outputs", fn: func() {}, outType: intType, wantErr: "provider must return"},
		{name: "too many outputs", fn: func() (int, error, error) { return 0, nil, nil }, outType: intType, wantErr: "provider must return"},
		{name: "second output not error", fn: func() (int, string) { return 0, "" }, outType: intType, wantErr: "second return value"},
		{name: "incompatible output", fn: func() string { return "x" }, outType: intType, wantErr: "not assignable/convertible"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := newFuncProvider(tc.fn, tc.outType)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestNewFuncProviderSuccess(t *testing.T) {
	intType := reflect.TypeOf(0)

	p, err := newFuncProvider(func() int { return 7 }, intType)
	require.NoError(t, err)
	require.NotNil(t, p)

	v, err := p.Provide(nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 7, v.(int))
}

func TestFuncProviderConvertibleReturn(t *testing.T) {
	p, err := newFuncProvider(func() int { return 9 }, reflect.TypeOf(int64(0)))
	require.NoError(t, err)

	v, err := p.Provide(nil, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(9), v.(int64))
}
