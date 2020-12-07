package types

import fmt "fmt"

// GetKind gets the session's kind
func (ws *WebSessionV2) GetKind() string {
	return ws.Kind
}

// GetName gets the session's name
func (ws *WebSessionV2) GetName() string {
	return ws.Metadata.Name
}

// GetUser returns the user this session is associated with
func (ws *WebSessionV2) GetUser() string {
	return ws.Spec.User
}

// String returns string representation of the session.
func (ws *WebSessionV2) String() string {
	return fmt.Sprintf("WebSession(kind=%v,name=%v,id=%v)", ws.GetKind(), ws.GetUser(), ws.GetName())
}
