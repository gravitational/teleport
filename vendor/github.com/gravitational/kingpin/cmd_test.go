package kingpin

import (
	"strings"

	"github.com/alecthomas/assert"

	"testing"
)

func parseAndExecute(app *Application, context *ParseContext) (string, error) {
	if err := parse(context, app); err != nil {
		return "", err
	}
	return app.execute(context)
}

func TestNestedCommands(t *testing.T) {
	app := New("app", "")
	sub1 := app.Command("sub1", "")
	sub1.Flag("sub1", "")
	subsub1 := sub1.Command("sub1sub1", "")
	subsub1.Command("sub1sub1end", "")

	sub2 := app.Command("sub2", "")
	sub2.Flag("sub2", "")
	sub2.Command("sub2sub1", "")

	context := tokenize([]string{"sub1", "sub1sub1", "sub1sub1end"}, false)
	selected, err := parseAndExecute(app, context)
	assert.NoError(t, err)
	assert.True(t, context.EOL())
	assert.Equal(t, "sub1 sub1sub1 sub1sub1end", selected)
}

func TestNestedCommandsWithArgs(t *testing.T) {
	app := New("app", "")
	cmd := app.Command("a", "").Command("b", "")
	a := cmd.Arg("a", "").String()
	b := cmd.Arg("b", "").String()
	context := tokenize([]string{"a", "b", "c", "d"}, false)
	selected, err := parseAndExecute(app, context)
	assert.NoError(t, err)
	assert.True(t, context.EOL())
	assert.Equal(t, "a b", selected)
	assert.Equal(t, "c", *a)
	assert.Equal(t, "d", *b)
}

func TestNestedCommandsWithFlags(t *testing.T) {
	app := New("app", "")
	cmd := app.Command("a", "").Command("b", "")
	a := cmd.Flag("aaa", "").Short('a').String()
	b := cmd.Flag("bbb", "").Short('b').String()
	err := app.init()
	assert.NoError(t, err)
	context := tokenize(strings.Split("a b --aaa x -b x", " "), false)
	selected, err := parseAndExecute(app, context)
	assert.NoError(t, err)
	assert.True(t, context.EOL())
	assert.Equal(t, "a b", selected)
	assert.Equal(t, "x", *a)
	assert.Equal(t, "x", *b)
}

func TestNestedCommandWithMergedFlags(t *testing.T) {
	app := New("app", "")
	cmd0 := app.Command("a", "")
	cmd0f0 := cmd0.Flag("aflag", "").Bool()
	// cmd1 := app.Command("b", "")
	// cmd1f0 := cmd0.Flag("bflag", "").Bool()
	cmd00 := cmd0.Command("aa", "")
	cmd00f0 := cmd00.Flag("aaflag", "").Bool()
	err := app.init()
	assert.NoError(t, err)
	context := tokenize(strings.Split("a aa --aflag --aaflag", " "), false)
	selected, err := parseAndExecute(app, context)
	assert.NoError(t, err)
	assert.True(t, *cmd0f0)
	assert.True(t, *cmd00f0)
	assert.Equal(t, "a aa", selected)
}

func TestNestedCommandWithDuplicateFlagErrors(t *testing.T) {
	app := New("app", "")
	app.Flag("test", "").Bool()
	app.Command("cmd0", "").Flag("test", "").Bool()
	err := app.init()
	assert.Error(t, err)
}

func TestNestedCommandWithArgAndMergedFlags(t *testing.T) {
	app := New("app", "")
	cmd0 := app.Command("a", "")
	cmd0f0 := cmd0.Flag("aflag", "").Bool()
	// cmd1 := app.Command("b", "")
	// cmd1f0 := cmd0.Flag("bflag", "").Bool()
	cmd00 := cmd0.Command("aa", "")
	cmd00a0 := cmd00.Arg("arg", "").String()
	cmd00f0 := cmd00.Flag("aaflag", "").Bool()
	err := app.init()
	assert.NoError(t, err)
	context := tokenize(strings.Split("a aa hello --aflag --aaflag", " "), false)
	selected, err := parseAndExecute(app, context)
	assert.NoError(t, err)
	assert.True(t, *cmd0f0)
	assert.True(t, *cmd00f0)
	assert.Equal(t, "a aa", selected)
	assert.Equal(t, "hello", *cmd00a0)
}

func TestDefaultSubcommandEOL(t *testing.T) {
	app := newTestApp()
	c0 := app.Command("c0", "").Default()
	c0.Command("c01", "").Default()
	c0.Command("c02", "")

	cmd, err := app.Parse([]string{"c0"})
	assert.NoError(t, err)
	assert.Equal(t, "c0 c01", cmd)
}

func TestDefaultSubcommandWithArg(t *testing.T) {
	app := newTestApp()
	c0 := app.Command("c0", "").Default()
	c01 := c0.Command("c01", "").Default()
	c012 := c01.Command("c012", "").Default()
	a0 := c012.Arg("a0", "").String()
	c0.Command("c02", "")

	cmd, err := app.Parse([]string{"c0", "hello"})
	assert.NoError(t, err)
	assert.Equal(t, "c0 c01 c012", cmd)
	assert.Equal(t, "hello", *a0)
}

func TestDefaultSubcommandWithFlags(t *testing.T) {
	app := newTestApp()
	c0 := app.Command("c0", "").Default()
	_ = c0.Flag("f0", "").Int()
	c0c1 := c0.Command("c1", "").Default()
	c0c1f1 := c0c1.Flag("f1", "").Int()
	selected, err := app.Parse([]string{"--f1=2"})
	assert.NoError(t, err)
	assert.Equal(t, "c0 c1", selected)
	assert.Equal(t, 2, *c0c1f1)
	_, err = app.Parse([]string{"--f2"})
	assert.Error(t, err)
}

func TestMultipleDefaultCommands(t *testing.T) {
	app := newTestApp()
	app.Command("c0", "").Default()
	app.Command("c1", "").Default()
	_, err := app.Parse([]string{})
	assert.Error(t, err)
}

func TestAliasedCommand(t *testing.T) {
	app := newTestApp()
	app.Command("one", "").Alias("two")
	selected, _ := app.Parse([]string{"one"})
	assert.Equal(t, "one", selected)
	selected, _ = app.Parse([]string{"two"})
	assert.Equal(t, "one", selected)
	// 2 due to "help" and "one"
	assert.Equal(t, 2, len(app.Model().FlattenedCommands()))
}

func TestDuplicateAlias(t *testing.T) {
	app := newTestApp()
	app.Command("one", "")
	app.Command("two", "").Alias("one")
	_, err := app.Parse([]string{"one"})
	assert.Error(t, err)
}
