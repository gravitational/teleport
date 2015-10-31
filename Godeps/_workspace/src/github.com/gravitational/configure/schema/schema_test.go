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
package schema

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/kr/pretty"
	"github.com/kylelemons/godebug/diff"

	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestConfig(t *testing.T) { TestingT(t) }

type ConfigSuite struct {
}

var _ = Suite(&ConfigSuite{})

func (s *ConfigSuite) TestParseTypes(c *C) {
	tcs := []struct {
		name   string
		cfg    string
		expect *Config
	}{
		{
			name: "string param",
			cfg: `{
                 "params": [
                     {
                       "name": "string1",
                       "type": "String"
                     }
                  ]
               }`,
			expect: &Config{
				Params: []Param{
					&StringParam{paramCommon{name: "string1"}, nil},
				},
			},
		},
		{
			name: "bool param",
			cfg: `{
                 "params": [
                     {
                       "name": "bool1",
                       "type": "Bool"
                     }
                  ]
               }`,
			expect: &Config{
				Params: []Param{
					&BoolParam{paramCommon{name: "bool1"}, nil},
				},
			},
		},
		{
			name: "enum param",
			cfg: `{
                 "params": [
                     {
                       "name": "enum1",
                       "type": "Enum",
                       "spec": {
                          "values": ["a", "b"]
                       }
                     }
                  ]
               }`,
			expect: &Config{
				Params: []Param{
					&EnumParam{paramCommon{name: "enum1"}, []string{"a", "b"}, nil},
				},
			},
		},
		{
			name: "key val param",
			cfg: `{
                 "params": [
                     {
                       "name": "kv1",
                       "type": "KeyVal",
                       "default": "path1:hello",
                       "required": true,
                       "spec": {
                          "separator": ":",
                          "keys": [
                              {"type": "Path", "name": "path1"},
                              {"type": "Path", "name": "path2"}
                          ]
                       }
                     }
                  ]
               }`,
			expect: &Config{
				Params: []Param{
					&KVParam{
						paramCommon{name: "kv1", req: true, def: "path1:hello"},
						":",
						[]Param{
							&PathParam{paramCommon{name: "path1"}, nil},
							&PathParam{paramCommon{name: "path2"}, nil},
						},
						nil,
					},
				},
			},
		},
		{
			name: "list key val param",
			cfg: `{
                 "params": [
                     {
                       "name": "mounts",
                       "type": "List",
                       "spec": {
                          "name": "volume",
                          "type": "KeyVal", 
                          "spec": {
                             "separator": ":",
                             "keys": [
                                 {"type": "Path", "name": "path1"},
                                 {"type": "Path", "name": "path2"}
                              ]
                          }
                       }
                     }
                  ]
               }`,
			expect: &Config{
				Params: []Param{
					&ListParam{
						paramCommon{name: "mounts"},
						&KVParam{
							paramCommon{name: "volume"},
							":",
							[]Param{
								&PathParam{paramCommon{name: "path1"}, nil},
								&PathParam{paramCommon{name: "path2"}, nil},
							},
							nil,
						},
						nil,
					},
				},
			},
		},
	}
	for i, tc := range tcs {
		comment := Commentf("test #%d (%v) cfg=%v, param=%v", i+1, tc.name, tc.cfg)
		cfg, err := ParseJSON(strings.NewReader(tc.cfg))
		c.Assert(err, IsNil, comment)
		c.Assert(len(cfg.Params), Equals, len(tc.expect.Params))
		for i, _ := range cfg.Params {
			c.Assert(cfg.Params[i], DeepEquals, tc.expect.Params[i], comment)
		}
	}
}

func (s *ConfigSuite) TestArgs(c *C) {
	tcs := []struct {
		name      string
		cfg       string
		expect    *Config
		expectErr bool
		args      []string
		vars      map[string]string
	}{
		{
			name: "string param",
			cfg: `{
                 "params": [
                     {
                       "name": "string1",
                       "type": "String"
                     }
                  ]
               }`,
			expect: &Config{
				Params: []Param{
					&StringParam{paramCommon{name: "string1"}, str("val1")},
				},
			},
			args: []string{"--string1", "val1"},
			vars: map[string]string{"STRING1": "val1"},
		},
		{
			name: "check defaults",
			cfg: `{
                 "params": [
                     {
                       "name": "string1",
                       "type": "String",
                       "default": "default value"
                     }
                  ]
               }`,
			expect: &Config{
				Params: []Param{
					&StringParam{
						paramCommon{name: "string1", def: "default value"},
						str("default value")},
				},
			},
			args: []string{},
			vars: map[string]string{"STRING1": "default value"},
		},
		{
			name: "check missing required param",
			cfg: `{
                 "params": [
                     {
                       "name": "string1",
                       "type": "String",
                       "required": true
                     }
                  ]
               }`,
			expectErr: true,
			args:      []string{},
		},
		{
			name: "check key values",
			cfg: `{
                 "params": [
                     {
                       "name": "volume",
                       "env": "PREFIX_VOLUME",
                       "type": "KeyVal",
                       "spec": {
                           "keys": [
                               {"name": "src", "type":"Path"},
                               {"name": "dst", "type":"Path"}
                           ]
                        }
                     }
                  ]
               }`,
			args: []string{"--volume", "/tmp/hello:/var/hello"},
			vars: map[string]string{"PREFIX_VOLUME": "/tmp/hello:/var/hello"},
			expect: &Config{
				Params: []Param{
					&KVParam{
						paramCommon{name: "volume", env: "PREFIX_VOLUME"},
						"",
						[]Param{
							&PathParam{paramCommon{name: "src"}, nil},
							&PathParam{paramCommon{name: "dst"}, nil},
						},
						[]Param{
							&PathParam{paramCommon{name: "src"}, str("/tmp/hello")},
							&PathParam{paramCommon{name: "dst"}, str("/var/hello")},
						},
					},
				},
			},
		},
		{
			name: "list of key values",
			cfg: `{
                 "params": [
                     {
                         "type": "List",
                         "name": "mounts",
                         "spec": {
                            "name": "volume",
                            "type": "KeyVal",
                            "spec": {
                              "keys": [
                                  {"name": "src", "type":"Path"},
                                  {"name": "dst", "type":"Path"}
                              ]
                            }
                        }
                     }
                  ]
               }`,
			args: []string{"--volume", "/tmp/hello:/var/hello"},
			vars: map[string]string{"VOLUME": "/tmp/hello:/var/hello"},
			expect: &Config{
				Params: []Param{
					&ListParam{
						paramCommon{name: "mounts"},
						&KVParam{
							paramCommon{name: "volume"},
							"",
							[]Param{
								&PathParam{paramCommon{name: "src"}, nil},
								&PathParam{paramCommon{name: "dst"}, nil},
							},
							nil,
						},
						[]Param{
							&KVParam{
								paramCommon{name: "volume"},
								"",
								[]Param{
									&PathParam{paramCommon{name: "src"}, nil},
									&PathParam{paramCommon{name: "dst"}, nil},
								},
								[]Param{
									&PathParam{paramCommon{name: "src"}, str("/tmp/hello")},
									&PathParam{paramCommon{name: "dst"}, str("/var/hello")},
								},
							},
						},
					},
				},
			},
		},
	}
	for i, tc := range tcs {
		comment := Commentf(
			"test #%d (%v) cfg=%v, args=%v", i+1, tc.name, tc.cfg, tc.args)
		cfg, err := ParseJSON(strings.NewReader(tc.cfg))
		c.Assert(err, IsNil, comment)

		if tc.expectErr {
			c.Assert(cfg.ParseArgs(tc.args), NotNil)
			continue
		}

		// make sure all the values have been parsed
		c.Assert(cfg.ParseArgs(tc.args), IsNil)
		c.Assert(len(cfg.Params), Equals, len(tc.expect.Params))
		for i, _ := range cfg.Params {
			comment := Commentf(
				"test #%d (%v) cfg=%v, args=%v\n%v",
				i+1, tc.name, tc.cfg, tc.args,
				diff.Diff(
					fmt.Sprintf("%# v", pretty.Formatter(cfg.Params[i])),
					fmt.Sprintf("%# v", pretty.Formatter(tc.expect.Params[i]))),
			)
			c.Assert(cfg.Params[i], DeepEquals, tc.expect.Params[i], comment)
		}

		// make sure args are equivalent to the passed arguments
		if len(tc.args) != 0 {
			args := cfg.Args()
			c.Assert(args, DeepEquals, tc.args, comment)
		}

		// make sure vars are what we expect them to be
		if len(tc.vars) != 0 {
			c.Assert(cfg.EnvVars(), DeepEquals, tc.vars, comment)
		}
	}
}

func (s *ConfigSuite) TestEnvVars(c *C) {
	tcs := []struct {
		name   string
		cfg    string
		expect map[string]string
	}{
		{
			name: "string param",
			cfg: `{
                 "params": [
                     {
                       "env": "ENV_STRING1",
                       "name": "string1",
                       "type": "String"
                     }
                  ]
               }`,
			expect: map[string]string{"ENV_STRING1": "val1"},
		},
		{
			name: "list of key values",
			cfg: `{
                 "params": [
                     {
                         "type": "List",
                         "name": "mounts",
                         "spec": {
                            "name": "volume",
                            "env": "PREFIX_VOLUME",
                            "type": "KeyVal",
                            "spec": {
                              "keys": [
                                  {"name": "src", "type":"Path"},
                                  {"name": "dst", "type":"Path"}
                              ]
                            }
                        }
                     }
                  ]
               }`,
			expect: map[string]string{
				"PREFIX_VOLUME": "/tmp/hello:/var/hello,/tmp/hello1:/var/hello2",
			},
		},
	}
	for i, tc := range tcs {
		comment := Commentf(
			"test #%d (%v) cfg=%v", i+1, tc.name, tc.cfg)
		cfg, err := ParseJSON(strings.NewReader(tc.cfg))
		c.Assert(err, IsNil, comment)

		os.Clearenv()
		for k, v := range tc.expect {
			os.Setenv(k, v)
		}
		c.Assert(cfg.ParseEnv(), IsNil)
		c.Assert(cfg.EnvVars(), DeepEquals, tc.expect, comment)
	}
}

func (s *ConfigSuite) TestParseVars(c *C) {
	tcs := []struct {
		name   string
		cfg    string
		expect map[string]string
	}{
		{
			name: "string param",
			cfg: `{
                 "params": [
                     {
                       "env": "ENV_STRING1",
                       "name": "string1",
                       "type": "String"
                     }
                  ]
               }`,
			expect: map[string]string{"string1": "val1"},
		},
		{
			name: "int param default",
			cfg: `{
                 "params": [
                     {
                       "name": "int1",
                       "type": "Int",
                       "default": "-1"
                     }
                  ]
               }`,
			expect: map[string]string{"int1": "-1"},
		},
		{
			name: "list of key values",
			cfg: `{
                 "params": [
                     {
                         "type": "List",
                         "name": "mounts",
                         "spec": {
                            "name": "volume",
                            "type": "KeyVal",
                            "spec": {
                              "keys": [
                                  {"name": "src", "type":"Path"},
                                  {"name": "dst", "type":"Path"}
                              ]
                            }
                        }
                     }
                  ]
               }`,
			expect: map[string]string{
				"mounts": "/tmp/hello:/var/hello,/tmp/hello1:/var/hello2",
			},
		},
	}
	for i, tc := range tcs {
		comment := Commentf(
			"test #%d (%v) cfg=%v", i+1, tc.name, tc.cfg)
		cfg, err := ParseJSON(strings.NewReader(tc.cfg))
		c.Assert(err, IsNil, comment)

		vars := make(map[string]string)
		for k, v := range tc.expect {
			vars[k] = v
		}
		c.Assert(cfg.ParseVars(vars), IsNil)
		c.Assert(cfg.Vars(), DeepEquals, tc.expect, comment)
	}
}

func str(val string) *string {
	return &val
}
