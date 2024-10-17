package oktaservice

import (
	"regexp"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

type oktaResourceItem struct {
	Name        string
	Description string
}

// filterOktaResources filters the Okta resources based on the provided filters.
// For instance  app-* will return all the groups that have a name starting with app-.
func filterOktaResources(filters []string, groups []*oktaResourceItem) ([]*oktaResourceItem, error) {
	if len(filters) == 0 {
		return groups, nil
	}
	f, err := getFilters(filters)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return getMatches(groups, f, func(a *oktaResourceItem) string { return a.Name }), nil
}

func getMatches[T any](resources []T, filters []*regexp.Regexp, getNameFn func(T) string) []T {
	var filteredResources []T
	for _, resource := range resources {
		for _, filter := range filters {
			if filter.MatchString(getNameFn(resource)) {
				filteredResources = append(filteredResources, resource)
				break
			}
		}
	}
	return filteredResources
}

func getFilters(filters []string) ([]*regexp.Regexp, error) {
	var compiledFilters []*regexp.Regexp
	for _, filter := range filters {
		compiledFilter, err := utils.CompileExpression(filter)
		if err != nil {
			return nil, trace.Wrap(err, "error compiling filter: %s", filter)
		}
		compiledFilters = append(compiledFilters, compiledFilter)
	}
	return compiledFilters, nil
}
