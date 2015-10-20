# Configure

Package configure generates configuration tools based on a struct
definition with tags. It can read a configuration for a struct
from YAML, environment variables and command line.

```go
// Given the struct definition:
   type Config struct {
     StringVar   string              `env:"TEST_STRING_VAR" cli:"string" yaml:"string"`
     BoolVar     bool                `env:"TEST_BOOL_VAR" cli:"bool" yaml:"bool"`
     IntVar      int                 `env:"TEST_INT_VAR" cli:"int" yaml:"int"`
     HexVar      hexType             `env:"TEST_HEX_VAR" cli:"hex" yaml:"hex"`
     MapVar      map[string]string   `env:"TEST_MAP_VAR" cli:"map" yaml:"map,flow"`
     SliceMapVar []map[string]string `env:"TEST_SLICE_MAP_VAR" cli:"slice" yaml:"slice,flow"`
  }
```

You can start initializing the struct from YAML, command line or environment.

```shell
# use godoc for more details
godoc github.com/gravitational/configure
```
