package bender

type Module interface {
	Configure(b *Binder)
}

type ModuleFunc func(b *Binder)

func (m ModuleFunc) Configure(b *Binder) {
	m(b)
}
