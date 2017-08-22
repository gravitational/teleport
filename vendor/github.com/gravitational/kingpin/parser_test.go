package kingpin

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/alecthomas/assert"
)

func TestParserExpandFromFile(t *testing.T) {
	f, err := ioutil.TempFile("", "")
	assert.NoError(t, err)
	defer os.Remove(f.Name())
	f.WriteString("hello\nworld\n")
	f.Close()

	app := New("test", "")
	arg0 := app.Arg("arg0", "").String()
	arg1 := app.Arg("arg1", "").String()

	_, err = app.Parse([]string{"@" + f.Name()})
	assert.NoError(t, err)
	assert.Equal(t, "hello", *arg0)
	assert.Equal(t, "world", *arg1)
}

func TestParseContextPush(t *testing.T) {
	app := New("test", "")
	app.Command("foo", "").Command("bar", "")
	c := tokenize([]string{"foo", "bar"}, false)
	a := c.Next()
	assert.Equal(t, TokenArg, a.Type)
	b := c.Next()
	assert.Equal(t, TokenArg, b.Type)
	c.Push(b)
	c.Push(a)
	a = c.Next()
	assert.Equal(t, "foo", a.Value)
	b = c.Next()
	assert.Equal(t, "bar", b.Value)
}
