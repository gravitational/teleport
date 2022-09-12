package auth

func Contains[T comparable](s []T, e T) bool {
	for _, v := range s {
		if v == e {
			return true
		}
	}
	return false
}

func Filter[T any](slice []T, f func(T) bool) []T {
	var n []T
	for _, e := range slice {
		if f(e) {
			n = append(n, e)
		}
	}
	return n
}

func Map[T any, R any](slice []T, f func(T) R) []R {
	n := make([]R, len(slice))

	for i, e := range slice {
		n[i] = f(e)
	}

	return n
}

func Find[T any](slice []T, f func(T) bool) *T {
	for _, e := range slice {
		if f(e) {
			return &e
		}
	}
	return nil
}

func Some[T any](slice []T, f func(T) bool) bool {
	for _, e := range slice {
		if f(e) {
			return true
		}
	}
	return false
}
