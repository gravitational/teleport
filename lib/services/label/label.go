package label

// LabelGetter allows retrieving a particular label by name or retreiving all
// labels at once. Prefer to use GetLabel when possible to avoid unnecessary
// copies.
type LabelGetter interface {
	GetLabel(key string) (value string, ok bool)
	GetAllLabels() map[string]string
}

type MapLabelGetter map[string]string

func (m MapLabelGetter) GetLabel(key string) (value string, ok bool) {
	v, ok := m[key]
	return v, ok
}

func (m MapLabelGetter) GetAllLabels() map[string]string {
	return map[string]string(m)
}
