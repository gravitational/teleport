package kingpin

import (
	"fmt"
	"os"
	"strings"
)

type flagGroup struct {
	short     map[string]*FlagClause
	long      map[string]*FlagClause
	flagOrder []*FlagClause
}

func newFlagGroup() *flagGroup {
	return &flagGroup{
		short: make(map[string]*FlagClause),
		long:  make(map[string]*FlagClause),
	}
}

func (f *flagGroup) merge(o *flagGroup) {
	for _, flag := range o.flagOrder {
		if flag.shorthand != 0 {
			f.short[string(flag.shorthand)] = flag
		}
		f.long[flag.name] = flag
		f.flagOrder = append(f.flagOrder, flag)
	}
}

// Flag defines a new flag with the given long name and help.
func (f *flagGroup) Flag(name, help string) *FlagClause {
	flag := newFlag(name, help)
	f.long[name] = flag
	f.flagOrder = append(f.flagOrder, flag)
	return flag
}

func (f *flagGroup) init(defaultEnvarPrefix string) error {
	for _, flag := range f.long {
		if defaultEnvarPrefix != "" && !flag.noEnvar && flag.envar == "" {
			flag.envar = envarTransform(defaultEnvarPrefix + "_" + flag.name)
		}
		if err := flag.init(); err != nil {
			return err
		}
		if flag.shorthand != 0 {
			f.short[string(flag.shorthand)] = flag
		}
	}
	return nil
}

func (f *flagGroup) parse(context *ParseContext) (*FlagClause, error) {
	var token *Token

loop:
	for {
		token = context.Peek()
		switch token.Type {
		case TokenEOL:
			break loop

		case TokenLong, TokenShort:
			flagToken := token
			defaultValue := ""
			var flag *FlagClause
			var ok bool
			invert := false

			name := token.Value
			if token.Type == TokenLong {
				if strings.HasPrefix(name, "no-") {
					name = name[3:]
					invert = true
				}
				flag, ok = f.long[name]
				if !ok {
					return nil, fmt.Errorf("unknown long flag '%s'", flagToken)
				}
			} else {
				flag, ok = f.short[name]
				if !ok {
					return nil, fmt.Errorf("unknown short flag '%s'", flagToken)
				}
			}

			context.Next()

			fb, ok := flag.value.(boolFlag)
			if ok && fb.IsBoolFlag() {
				if invert {
					defaultValue = "false"
				} else {
					defaultValue = "true"
				}
			} else {
				if invert {
					context.Push(token)
					return nil, fmt.Errorf("unknown long flag '%s'", flagToken)
				}
				token = context.Peek()
				if token.Type != TokenArg {
					context.Push(token)
					return nil, fmt.Errorf("expected argument for flag '%s'", flagToken)
				}
				context.Next()
				defaultValue = token.Value
			}

			context.matchedFlag(flag, defaultValue)
			return flag, nil

		default:
			break loop
		}
	}
	return nil, nil
}

func (f *flagGroup) visibleFlags() int {
	count := 0
	for _, flag := range f.long {
		if !flag.hidden {
			count++
		}
	}
	return count
}

// FlagClause is a fluid interface used to build flags.
type FlagClause struct {
	parserMixin
	actionMixin
	name         string
	shorthand    byte
	help         string
	envar        string
	noEnvar      bool
	defaultValue string
	placeholder  string
	hidden       bool
}

func newFlag(name, help string) *FlagClause {
	f := &FlagClause{
		name: name,
		help: help,
	}
	return f
}

func (f *FlagClause) needsValue() bool {
	return f.required && f.defaultValue == ""
}

func (f *FlagClause) formatPlaceHolder() string {
	if f.placeholder != "" {
		return f.placeholder
	}
	if f.defaultValue != "" {
		if _, ok := f.value.(*stringValue); ok {
			return fmt.Sprintf("%q", f.defaultValue)
		}
		return f.defaultValue
	}
	return strings.ToUpper(f.name)
}

func (f *FlagClause) init() error {
	if f.required && f.defaultValue != "" {
		return fmt.Errorf("required flag '--%s' with default value that will never be used", f.name)
	}
	if f.value == nil {
		return fmt.Errorf("no type defined for --%s (eg. .String())", f.name)
	}
	if !f.noEnvar && f.envar != "" {
		if v := os.Getenv(f.envar); v != "" {
			f.defaultValue = v
		}
	}
	return nil
}

// Dispatch to the given function after the flag is parsed and validated.
func (f *FlagClause) Action(action Action) *FlagClause {
	f.addAction(action)
	return f
}

func (f *FlagClause) PreAction(action Action) *FlagClause {
	f.addPreAction(action)
	return f
}

// Default value for this flag. It *must* be parseable by the value of the flag.
func (f *FlagClause) Default(value string) *FlagClause {
	f.defaultValue = value
	return f
}

// DEPRECATED: Use Envar(name) instead.
func (f *FlagClause) OverrideDefaultFromEnvar(envar string) *FlagClause {
	return f.Envar(envar)
}

// Envar overrides the default value for a flag from an environment variable,
// if it is set.
func (f *FlagClause) Envar(name string) *FlagClause {
	f.envar = name
	f.noEnvar = false
	return f
}

// NoEnvar forces environment variable defaults to be disabled for this flag.
// Most useful in conjunction with app.DefaultEnvars().
func (f *FlagClause) NoEnvar() *FlagClause {
	f.envar = ""
	f.noEnvar = true
	return f
}

// PlaceHolder sets the place-holder string used for flag values in the help. The
// default behaviour is to use the value provided by Default() if provided,
// then fall back on the capitalized flag name.
func (f *FlagClause) PlaceHolder(placeholder string) *FlagClause {
	f.placeholder = placeholder
	return f
}

// Hidden hides a flag from usage but still allows it to be used.
func (f *FlagClause) Hidden() *FlagClause {
	f.hidden = true
	return f
}

// Required makes the flag required. You can not provide a Default() value to a Required() flag.
func (f *FlagClause) Required() *FlagClause {
	f.required = true
	return f
}

// Short sets the short flag name.
func (f *FlagClause) Short(name byte) *FlagClause {
	f.shorthand = name
	return f
}

// Bool makes this flag a boolean flag.
func (f *FlagClause) Bool() (target *bool) {
	target = new(bool)
	f.SetValue(newBoolValue(target))
	return
}
