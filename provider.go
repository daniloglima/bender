package bender

import (
	"fmt"
	"reflect"
)

type Provider interface {
	Provide(c *Container, st *resolveState) (any, error)
}

type instanceProvider struct {
	v any
}

func (p instanceProvider) Provide(_ *Container, _ *resolveState) (any, error) {
	return p.v, nil
}

type funcProvider struct {
	fn       reflect.Value
	fnType   reflect.Type
	outType  reflect.Type
	hasError bool
}

func newFuncProvider(fn any, outType reflect.Type) (Provider, error) {
	v := reflect.ValueOf(fn)
	if v.Kind() != reflect.Func {
		return nil, fmt.Errorf("provider must be a function, got %T", fn)
	}

	t := v.Type()
	if t.NumOut() == 0 || t.NumOut() > 2 {
		return nil, fmt.Errorf("provider must return (T) or (T, error)")
	}

	if t.NumOut() == 2 {
		if !t.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
			return nil, fmt.Errorf("second return value must be of type error")
		}
	}

	if !t.Out(0).AssignableTo(outType) && !t.Out(0).ConvertibleTo(outType) {
		return nil, fmt.Errorf("provider return type %s is not assignable/convertible to %s", t.Out(0), outType)
	}

	return &funcProvider{
		fn:       v,
		fnType:   t,
		outType:  outType,
		hasError: t.NumOut() == 2,
	}, nil

}

func (p *funcProvider) Provide(c *Container, st *resolveState) (any, error) {
	args := make([]reflect.Value, 0, p.fnType.NumIn())
	for i := 0; i < p.fnType.NumIn(); i++ {
		pt := p.fnType.In(i)
		v, err := c.resolve(keyOfType(pt, ""), st)
		if err != nil {
			return nil, err
		}
		rv := reflect.ValueOf(v)

		// ensure we can pass it to the function param
		if !rv.IsValid() {
			return nil, fmt.Errorf("resolved invalid value for %s", pt)
		}

		if rv.Type().AssignableTo(pt) {
			args = append(args, rv)
			continue
		}

		if rv.Type().ConvertibleTo(pt) {
			args = append(args, rv.Convert(pt))
			continue
		}

		return nil, fmt.Errorf("resolved %s but cannot pass to param %s", rv.Type(), pt)
	}

	out := p.fn.Call(args)

	if p.hasError {
		if !out[1].IsNil() {
			return nil, out[1].Interface().(error)
		}
	}

	val := out[0]

	// assign to requested outType
	if val.Type().AssignableTo(p.outType) {
		return val.Interface(), nil
	}

	if val.Type().ConvertibleTo(p.outType) {
		return val.Convert(p.outType).Interface(), nil
	}

	// shouldn't happen due to validation in NewFuncProvider
	return nil, fmt.Errorf("provider returned %s not compatible with %s", val.Type(), p.outType)
}
