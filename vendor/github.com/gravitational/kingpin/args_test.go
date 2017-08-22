package kingpin

import (
	"io/ioutil"
	"testing"

	"github.com/alecthomas/assert"
)

func TestArgRemainder(t *testing.T) {
	app := New("test", "")
	v := app.Arg("test", "").Strings()
	args := []string{"hello", "world"}
	_, err := app.Parse(args)
	assert.NoError(t, err)
	assert.Equal(t, args, *v)
}

func TestArgRemainderErrorsWhenNotLast(t *testing.T) {
	a := newArgGroup()
	a.Arg("test", "").Strings()
	a.Arg("test2", "").String()
	assert.Error(t, a.init())
}

func TestArgMultipleRequired(t *testing.T) {
	terminated := false
	app := New("test", "")
	app.Version("0.0.0").Writer(ioutil.Discard)
	app.Arg("a", "").Required().String()
	app.Arg("b", "").Required().String()
	app.Terminate(func(int) { terminated = true })

	_, err := app.Parse([]string{})
	assert.Error(t, err)
	_, err = app.Parse([]string{"A"})
	assert.Error(t, err)
	_, err = app.Parse([]string{"A", "B"})
	assert.NoError(t, err)
	_, err = app.Parse([]string{"--version"})
	assert.True(t, terminated)
}

func TestInvalidArgsDefaultCanBeOverridden(t *testing.T) {
	app := New("test", "")
	app.Arg("a", "").Default("invalid").Bool()
	_, err := app.Parse([]string{})
	assert.Error(t, err)
}

func TesArgtMultipleValuesDefault(t *testing.T) {
	app := New("test", "")
	a := app.Arg("a", "").Default("default1", "default2").Strings()
	_, err := app.Parse([]string{})
	assert.NoError(t, err)
	assert.Equal(t, []string{"default1", "default2"}, *a)
}
