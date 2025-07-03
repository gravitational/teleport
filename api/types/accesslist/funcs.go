package accesslist

func isZero[T comparable](t T) bool {
	var zero T
	return t == zero
}
