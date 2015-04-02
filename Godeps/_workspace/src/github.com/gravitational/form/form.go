// Form is a minimalist HTTP web form parser library based on functional arguments.
package form

import (
	"fmt"
	"net/http"
	"time"

	"strconv"
)

// Param is a functional argument parameter passed to the Parse function
type Param func(r *http.Request) error

// Parse takes http.Request and form arguments that it needs to extract
//
//   import (
//        "github.com/gravitational/form"
//   )
//
//   var duration time.Duration
//   var count int
//   name := "default" // a simple way to set default argument
//
//   err := form.Parse(r,
//      form.Duration("duration", &duration),
//      form.Int("count", &count, Required()), // notice the "Required" argument
//      form.String("name", &name),
//      )
//
//   if err != nil {
//        // handle error here
//   }
func Parse(r *http.Request, params ...Param) error {
	if err := r.ParseForm(); err != nil {
		return err
	}
	for _, p := range params {
		if err := p(r); err != nil {
			return err
		}
	}
	return nil
}

// Duration extracts duration expressed as a Go duration string e.g. "1s"
func Duration(name string, out *time.Duration, predicates ...Predicate) Param {
	return func(r *http.Request) error {
		for _, p := range predicates {
			if err := p.Pass(name, r); err != nil {
				return err
			}
		}
		v := r.PostForm.Get(name)
		if v == "" {
			return nil
		}
		d, err := time.ParseDuration(v)
		if err != nil {
			return &BadParameterError{Param: name, Message: err.Error()}
		}
		*out = d
		return nil
	}
}

// String extracts the argument by name as is without any changes
func String(name string, out *string, predicates ...Predicate) Param {
	return func(r *http.Request) error {
		for _, p := range predicates {
			if err := p.Pass(name, r); err != nil {
				return err
			}
		}
		*out = r.PostForm.Get(name)
		return nil
	}
}

// Int extracts the integer argument in decimal format e.g. "10"
func Int(name string, out *int, predicates ...Predicate) Param {
	return func(r *http.Request) error {
		v := r.PostForm.Get(name)
		for _, p := range predicates {
			if err := p.Pass(name, r); err != nil {
				return err
			}
		}
		if v == "" {
			return nil
		}
		p, err := strconv.Atoi(v)
		if err != nil {
			return &BadParameterError{Param: name, Message: err.Error()}
		}
		*out = p
		return nil
	}
}

// Predicate provides an extensible way to check various conditions on a variable
// e.g. setting minimums and maximums, or parsing some regular expressions
type Predicate interface {
	Pass(param string, r *http.Request) error
}

// PredicateFunc takes a func and converts it into a Predicate-compatible interface
type PredicateFunc func(param string, r *http.Request) error

func (p PredicateFunc) Pass(param string, r *http.Request) error {
	return p(param, r)
}

// Required checker parameter ensures that the parameter is indeed supplied by user
// it returns MissingParameterError when parameter is not present
func Required() Predicate {
	return PredicateFunc(func(param string, r *http.Request) error {
		if r.PostForm.Get(param) == "" {
			return &MissingParameterError{Param: param}
		}
		return nil
	})
}

// MissingParameterError is an error that indicates that required parameter was not
// supplied by user.
type MissingParameterError struct {
	Param string
}

func (p *MissingParameterError) Error() string {
	return fmt.Sprintf("missing required parameter: '%v'", p.Param)
}

// BadParameterError is returned whenever the parameter format does not match
// required restrictions.
type BadParameterError struct {
	Param   string // Param is a paramter name
	Message string // Message is an error message presented to user
}

func (p *BadParameterError) Error() string {
	return fmt.Sprintf("bad parameter '%v', error: %v", p.Param, p.Message)
}
