package utils

// Combinations yields all unique sub-slices of the input slice.
func Combinations(verbs []string) [][]string {
	var result [][]string
	length := len(verbs)

	for i := 0; i < (1 << length); i++ {
		subslice := make([]string, 0)
		for j := 0; j < length; j++ {
			if i&(1<<j) != 0 {
				subslice = append(subslice, verbs[j])
			}
		}
		result = append(result, subslice)
	}

	return result
}
