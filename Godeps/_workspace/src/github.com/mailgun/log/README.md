log
===

Go logging library used at Mailgun.

Usage
-----

Define a logging configuration in a YAML config file. Currently "console" and "syslog" loggers are supported (you can omit one or another):

```yaml
logging:
  - name: console
  - name: syslog
```

Logging config can be built into your program's config struct:

```go
import "github.com/mailgun/log"


type Config struct {
  // some program-specific configuration

  // logging configuration
  Logging []*log.LogConfig
}
```

After config parsing, initialize the logging library:

```go
import (
  "github.com/mailgun/cfg"
  "github.com/mailgun/log"
)

func main() {
  conf := Config{}

  // parse config with logging configuration
  cfg.LoadConfig("path/to/config.yaml", &conf)

  // init the logging package
  log.Init(conf.Logging)
}
```
