package mock

type Reason struct {
	Text string `json:"Text"`
}

// ErrorPayload represents an error returned by the API.
type ErrorPayload struct {
	Reason Reason `json:"Reason"`
}

func newUnauthenticatedError(reason string) *ErrorPayload {
	return &ErrorPayload{
		Reason: Reason{
			Text: "unauthenticated: " + reason,
		},
	}
}

func newBadRequestError(reason string) *ErrorPayload {
	return &ErrorPayload{
		Reason: Reason{
			Text: "bad request: " + reason,
		},
	}
}
