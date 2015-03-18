// Form is a prototype for the form parser library based on functional arguments
// it's an early stage, so I'm keeping it here
package form

import (
	"fmt"
	"net/http"
	"time"

	"strconv"
)

// Param is a functional argument parameter passed to the Parse function
type Param func(r *http.Request) error

// Parse parses the form using Params passed in.
// E.g.
//
// var days int
// err := Parse(r, Int("days", &days))
//
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
			return err
		}
		*out = p
		return nil
	}
}

type Predicate interface {
	Pass(param string, r *http.Request) error
}

type PredicateFunc func(param string, r *http.Request) error

func (p PredicateFunc) Pass(param string, r *http.Request) error {
	return p(param, r)
}

func Required() Predicate {
	return PredicateFunc(func(param string, r *http.Request) error {
		if r.PostForm.Get(param) == "" {
			return &MissingParameterError{Param: param}
		}
		return nil
	})
}

type MissingParameterError struct {
	Param string
}

func (p *MissingParameterError) Error() string {
	return fmt.Sprintf("missing required parameter: '%v'", p.Param)
}

type BadParameterError struct {
	Param   string
	Message string
}

func (p *BadParameterError) Error() string {
	return fmt.Sprintf("bad parameter '%v', error: %v", p.Param, p.Message)
}
