package configure

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"text/template"

	"github.com/gravitational/configure/cstrings"
	"github.com/gravitational/trace"
)

func renderTemplate(data []byte) ([]byte, error) {
	t := template.New("tpl")
	c, err := newCtx()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	t.Funcs(map[string]interface{}{
		"env":  c.Env,
		"file": c.File,
	})
	t, err = t.Parse(string(data))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	buf := &bytes.Buffer{}
	if err := t.Execute(buf, nil); err != nil {
		return nil, trace.Wrap(err)
	}
	return buf.Bytes(), nil
}

func newCtx() (*ctx, error) {
	values := os.Environ()
	c := &ctx{
		env: make(map[string]string, len(values)),
	}
	for _, v := range values {
		vals := strings.SplitN(v, "=", 2)
		if len(vals) != 2 {
			return nil, trace.Errorf("failed to parse variable: '%v'", v)
		}
		c.env[vals[0]] = vals[1]
	}
	return c, nil
}

type ctx struct {
	env map[string]string
}

func (c *ctx) File(path string) (string, error) {
	o, err := ioutil.ReadFile(path)
	if err != nil {
		return "", trace.Wrap(err, fmt.Sprintf("reading file: %v", path))
	}
	return string(o), nil
}

func (c *ctx) Env(key string) (string, error) {
	v, ok := c.env[key]
	if !ok {
		return "", trace.Errorf("environment variable '%v' is not set", key)
	}
	values := cstrings.SplitComma(v)
	out := make([]string, len(values))
	for i, p := range values {
		out[i] = quoteYAML(p)
	}
	return strings.Join(out, ","), nil
}

func quoteYAML(val string) string {
	if len(val) == 0 {
		return val
	}
	if strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'") {
		return val
	}
	if strings.ContainsAny(val, ":") {
		return "'" + val + "'"
	}
	return val
}
