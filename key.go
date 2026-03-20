package bender

import "reflect"

type Key struct {
	t    reflect.Type
	name string
}

func keyOfType(t reflect.Type, name string) Key {
	return Key{t: t, name: name}
}
