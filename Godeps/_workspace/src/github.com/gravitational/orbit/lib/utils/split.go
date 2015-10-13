package utils

// SplitAt splits array of strings at a given separator
func SplitAt(args []string, sep string) ([]string, []string) {
	index := -1
	for i, a := range args {
		if a == sep {
			index = i
			break
		}
	}
	if index == -1 {
		return args, []string{}
	}
	return args[:index], args[index+1:]
}
