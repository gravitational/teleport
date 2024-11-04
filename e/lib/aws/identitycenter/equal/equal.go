package equal

import (
	"reflect"
	"slices"
	"sort"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
)

func metadataEqual(a, b *headerv1.Metadata) bool {
	return a.GetName() == b.GetName() &&
		a.GetNamespace() == b.GetNamespace() &&
		a.GetDescription() == b.GetDescription() &&
		reflect.DeepEqual(a.GetLabels(), b.GetLabels())
}

func compareStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	sort.Strings(a)
	sort.Strings(b)
	return slices.Equal(a, b)
}
