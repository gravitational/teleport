package main

import (
	"flag"
	"fmt"
	"strings"

	nostructfieldassign "github.com/gravitational/teleport/build.assets/tools/linters/nostructfieldassign"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(newAnalyzer())
}

func newAnalyzer() *analysis.Analyzer {
	var rules ruleFlag

	analyzer := nostructfieldassign.NewAnalyzer()
	analyzer.Flags = *flag.NewFlagSet(analyzer.Name, flag.ContinueOnError)
	analyzer.Flags.Var(&rules, "fields",
		"comma-separated list of forbidden struct fields in the form <pkgPath>.<Type>.<Field>")
	analyzer.Run = func(pass *analysis.Pass) (any, error) {
		return nostructfieldassign.NewAnalyzer(rules...).Run(pass)
	}

	return analyzer
}

type ruleFlag []nostructfieldassign.Rule

func (f *ruleFlag) String() string {
	var b strings.Builder
	for i, rule := range *f {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(rule.Package)
		b.WriteByte('.')
		b.WriteString(rule.Type)
		b.WriteByte('.')
		b.WriteString(rule.Field)
		if rule.ErrorMessage != "" {
			b.WriteString("# ")
			b.WriteString(rule.ErrorMessage)
		}
	}
	return b.String()
}

func (f *ruleFlag) Set(s string) error {
	for _, entry := range splitRuleSpecs(s) {
		entry = strings.TrimSpace(entry)
		if strings.Trim(entry, " \t\r\n,") == "" {
			continue
		}
		rule, err := parseFieldKey(entry)
		if err != nil {
			return err
		}
		*f = append(*f, rule)
	}
	return nil
}

func splitRuleSpecs(s string) []string {
	var specs []string
	start := 0

	for i := 0; i < len(s); i++ {
		if s[i] != ',' {
			continue
		}
		if !looksLikeRuleStart(s[i+1:]) {
			continue
		}

		specs = append(specs, s[start:i])
		start = i + 1
	}

	specs = append(specs, s[start:])
	return specs
}

// looksLikeRuleStart reports whether s begins with another rule spec rather
// than continuing the current rule's error message. It is used to distinguish
// commas that separate rules from commas that appear inside "#"-suffix text by
// checking whether the next token has the expected "<pkgPath>.<Type>.<Field>"
// shape before any subsequent "#" or "," delimiter.
func looksLikeRuleStart(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}

	end := len(s)
	if idx := strings.IndexAny(s, "#,"); idx >= 0 {
		end = idx
	}

	candidate := strings.TrimSpace(s[:end])
	lastDot := strings.LastIndex(candidate, ".")
	if lastDot < 0 {
		return false
	}
	fieldName := candidate[lastDot+1:]
	rest := candidate[:lastDot]
	if fieldName == "" {
		return false
	}

	secondLastDot := strings.LastIndex(rest, ".")
	if secondLastDot < 0 {
		return false
	}

	typeName := rest[secondLastDot+1:]
	pkgPath := rest[:secondLastDot]
	return pkgPath != "" && typeName != ""
}

func parseFieldKey(s string) (nostructfieldassign.Rule, error) {
	var msg string
	if idx := strings.Index(s, "#"); idx >= 0 {
		msg = strings.TrimSpace(s[idx+1:])
		s = strings.TrimSpace(s[:idx])
	}

	lastDot := strings.LastIndex(s, ".")
	if lastDot < 0 {
		return nostructfieldassign.Rule{}, fmt.Errorf("nostructfieldassign: invalid field spec %q: expected <pkgPath>.<Type>.<Field>", s)
	}
	fieldName := s[lastDot+1:]
	rest := s[:lastDot]

	secondLastDot := strings.LastIndex(rest, ".")
	if secondLastDot < 0 {
		return nostructfieldassign.Rule{}, fmt.Errorf("nostructfieldassign: invalid field spec %q: expected <pkgPath>.<Type>.<Field>", s)
	}
	typeName := rest[secondLastDot+1:]
	pkgPath := rest[:secondLastDot]

	if pkgPath == "" || typeName == "" || fieldName == "" {
		return nostructfieldassign.Rule{}, fmt.Errorf("nostructfieldassign: invalid field spec %q: pkgPath, type, and field must all be non-empty", s)
	}

	return nostructfieldassign.Rule{
		Package:      pkgPath,
		Type:         typeName,
		Field:        fieldName,
		ErrorMessage: msg,
	}, nil
}
