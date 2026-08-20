package fixtures

import "flag"

var (
	SSHNode = register("ssh-node")
	// SSHNodeBPF runs a second node, docker-node-bpf, with Enhanced Session Recording enabled.
	SSHNodeBPF = register("ssh-node-bpf")
	Kube       = register("kube")
	Connect    = register("connect")
)

type Fixture struct {
	Name    string
	Enabled bool
}

func (f *Fixture) String() string {
	return f.Name
}

var all []*Fixture

func register(name string) *Fixture {
	f := &Fixture{Name: name}

	all = append(all, f)

	return f
}

func All() []*Fixture {
	return all
}

func Enabled() []*Fixture {
	var enabled []*Fixture
	for _, f := range all {
		if f.Enabled {
			enabled = append(enabled, f)
		}
	}
	return enabled
}

func BindFlags(fs *flag.FlagSet) {
	for _, f := range all {
		fs.BoolVar(&f.Enabled, "with-"+f.Name, false, "enable the "+f.Name+" fixture")
	}
}

func FindByName(name string) *Fixture {
	for _, f := range all {
		if f.Name == name {
			return f
		}
	}

	return nil
}
