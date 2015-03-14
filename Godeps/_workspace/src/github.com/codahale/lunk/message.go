package lunk

// Message returns an Event which contains only a human-readable message.
func Message(msg string) Event {
	return messageEvent{Message: msg}
}

type messageEvent struct {
	Message string `json:"msg"`
}

func (messageEvent) Schema() string {
	return "message"
}
