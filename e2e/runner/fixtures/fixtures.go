/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package fixtures

import "flag"

var (
	SSHNode = register("ssh-node")
	Connect = register("connect")
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
