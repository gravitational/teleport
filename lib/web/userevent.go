package web

import (
	"fmt"
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/lib/httplib"
)

// CreateUserEventRequest contains the event and properties associated with a user event
// the usageReporter convert event function will later set the timestamp
// and anonymize/set the cluster name
type CreateUserEventRequest struct {
	// Event describes the event being captured
	Event string `json:"event"`
	// Alert is a banner click event property
	Alert string `json:"alert"`
}

// CheckAndSetDefaults validates the Request has the required fields.
func (r *CreateUserEventRequest) CheckAndSetDefaults() error {
	if r.Event == "" {
		return trace.BadParameter("missing required parameter Event")
	}

	return nil
}

// createUserEventHandle sends a user event to the UserEvent service
func (h *Handler) createUserEventHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	var req CreateUserEventRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	// todo mberg integrate with Tim Bs prehog work
	fmt.Printf("** req: %v", req)
	return nil, nil
}
