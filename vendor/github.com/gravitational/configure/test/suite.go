/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package test

import (
	"encoding/hex"

	. "gopkg.in/check.v1"
)

type ConfigSuite struct {
}

func (s *ConfigSuite) CheckVariables(c *C, cfg *Config) {
	c.Assert(cfg.StringVar, Equals, "string1")
	c.Assert(cfg.BoolVar, Equals, true)
	c.Assert(cfg.IntVar, Equals, -1)
	c.Assert(cfg.HexVar, Equals, hexType("hexvar"))
	c.Assert(cfg.MapVar, DeepEquals, map[string]string{"a": "b", "c": "d", "e": "f"})
	c.Assert(cfg.SliceMapVar, DeepEquals, []map[string]string{{"a": "b", "c": "d"}, {"e": "f"}})
	c.Assert(cfg.Nested.NestedVar, Equals, "nested")
}

type Config struct {
	StringVar   string              `env:"TEST_STRING_VAR" cli:"string" yaml:"string"`
	BoolVar     bool                `env:"TEST_BOOL_VAR" cli:"bool" yaml:"bool"`
	IntVar      int                 `env:"TEST_INT_VAR" cli:"int" yaml:"int"`
	HexVar      hexType             `env:"TEST_HEX_VAR" cli:"hex" yaml:"hex"`
	MapVar      map[string]string   `env:"TEST_MAP_VAR" cli:"map" yaml:"map,flow"`
	SliceMapVar []map[string]string `env:"TEST_SLICE_MAP_VAR" cli:"slice" yaml:"slice,flow"`
	Nested      struct {
		NestedVar string `env:"TEST_NESTED_VAR" cli:"nested" yaml:"nested"`
	} `yaml:"nested"`
}

type hexType string

func (t *hexType) SetString(v string) error {
	data, err := hex.DecodeString(v)
	if err != nil {
		return err
	}
	*t = hexType(data)
	return nil
}

func (t *hexType) SetEnv(v string) error {
	return t.SetString(v)
}

func (t *hexType) SetCLI(v string) error {
	return t.SetString(v)
}

func (t *hexType) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var val string
	if err := unmarshal(&val); err != nil {
		return err
	}
	return t.SetString(val)
}
