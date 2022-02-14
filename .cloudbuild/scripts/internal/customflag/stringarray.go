package customflag

import "strings"

type StringArray []string

func (flags StringArray) String() string {
	return strings.Join(flags, ", ")
}

func (flags *StringArray) Set(value string) error {
	*flags = append(*flags, value)
	return nil
}
