package mypackage

type Object interface {
	GetName() string
}

type Parent struct {
	Id   int
	Name string
}

func (p Parent) GetName() string {
	return p.Name
}

func (p *Parent) SetName(name string) {
	p.Name = name
}

type Child struct {
	Parent
	OtherField string
}
