# Configure

`configure` is a golang library that populates a struct from environment variables, command line arguments and YAML files.
It works by reading a struct definition with special tags. 

### Usage 

The latest can be seen if you run:
```
godoc github.com/gravitational/configure
```

But here's a quickstart: Define a sample structure, for example:
```go

	 type Config struct {
	   StringVar   string              `env:"STRING_VAR" cli:"string-var" yaml:"string_var"`
	   BoolVar     bool                `env:"BOOL_VAR" cli:"bool_var" yaml:"bool_var"`
	   IntVar      int                 `env:"INT_VAR" cli:"int_var" yaml:"int_var"`
	   HexVar      hexType             `env:"HEX_VAR" cli:"hex_var" yaml:"hex_var"`
	   MapVar      map[string]string   `env:"MAP_VAR" cli:"map_var" yaml:"map_var,flow"`
	   SliceMapVar []map[string]string `env:"SLICE_MAP_VAR" cli:"slice_var" yaml:"slice_var,flow"`
	}
```

Then you can query the environment and populate that structure from environment variables, YAML files or command line arguments.

```go
	import (
	   "os"
	   "github.com/gravitational/configure"
	)

	func main() {
	   var cfg Config
	   // parse environment variables
	   err := configure.ParseEnv(&cfg)
	   // parse YAML
	   err = configure.ParseYAML(&cfg)
	   // parse command line arguments
	   err = configure.ParseCommandLine(&cfg, os.Ars[1:])
	}
```
