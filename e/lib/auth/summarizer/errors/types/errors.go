package types

type BadResponseError struct {
	Message string
}

func (e BadResponseError) Error() string {
	return e.Message
}
