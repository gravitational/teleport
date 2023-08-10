package eventschema

type stringSet map[string]struct{}

var ignoredFields = map[string]stringSet{}
