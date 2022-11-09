package web

import (
	"fmt"
	"net/http"

	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

// CreateUserEventRequest contains the event and properties associated with a user event
// the usageReporter convert event function will later set the timestamp
// and anonymize/set the cluster name
type CreateUserEventRequest struct {
	// Event describes the event being captured
	Event string `json:"event"`
	// Properties a key value set of event metadata.
	Properties map[string]interface{} `json:"properties"`
}

// CheckAndSetDefaults validates the Request has the required fields.
func (r *CreateUserEventRequest) CheckAndSetDefaults() error {
	if r.Event == "" {
		return trace.BadParameter("missing required parameter Event")
	}

	if r.Properties == nil {
		r.Properties = make(map[string]interface{})
	}

	r.Properties["trial"] = modules.GetModules().CloudTrial()
	r.Properties["cloud"] = modules.GetModules().Features().Cloud

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
