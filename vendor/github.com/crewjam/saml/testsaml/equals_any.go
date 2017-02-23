package testsaml

import (
	"strings"

	"gopkg.in/check.v1"
)

var EqualsAny = equalsAny{}

type equalsAny struct {
}

func (checker equalsAny) Info() *check.CheckerInfo {
	return &check.CheckerInfo{
		Name:   "EqualsAny",
		Params: []string{"value", "expected"},
	}
}

func (checker equalsAny) Check(params []interface{}, names []string) (result bool, error string) {
	errors := []string{}
	for _, param := range params[1].([]interface{}) {
		r, e := check.Equals.Check([]interface{}{params[0], param}, names)
		if r {
			return true, ""
		}
		errors = append(errors, e)
	}
	return false, strings.Join(errors, "\n")
}
