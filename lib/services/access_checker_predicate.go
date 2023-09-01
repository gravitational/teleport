package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

type accessCheckerTAG struct {
	info         *AccessInfo
	localCluster string
	tagEndpoint  string
	*accessChecker
}

// CheckAccess checks access to the specified resource.
func (a *accessCheckerTAG) CheckAccess(r AccessCheckable, state AccessState, matchers ...RoleMatcher) error {
	// TAG only supports servers, everything else is handled by the RBAC access checker
	if _, ok := r.(types.Server); !ok {
		return a.accessChecker.CheckAccess(r, state, matchers...)
	}

	u, err := url.Parse(a.tagEndpoint)
	if err != nil {
		return trace.Wrap(err)
	}

	login := ""
	// this is a hack to get the login from the matchers
	for _, matcher := range matchers {
		if l, ok := matcher.(*loginMatcher); ok {
			login = l.login
			break
		}
	}

	u.Path = "/api/v1/authorize"
	q := u.Query()
	q.Add("target", r.GetName())
	q.Add("user", a.info.Username)
	if login != "" {
		q.Add("login", login)
	}
	u.RawQuery = q.Encode()
	resp, err := http.Get(u.String())
	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return trace.Wrap(err)
	}
	if string(body) == "True" {
		return nil
	}

	return trace.AccessDenied("No access")
}

// CheckAgentForward checks if the role can request agent forward for this
// user.
func (a *accessCheckerTAG) CheckAgentForward(login string) error {
	return nil
}

// CanForwardAgents returns true if this role set offers capability to forward
// agents.
func (a *accessCheckerTAG) CanForwardAgents() bool {
	return false
}

// CanPortForward returns true if this RoleSet can forward ports.
func (a *accessCheckerTAG) CanPortForward() bool {
	return false
}

// PermitX11Forwarding returns true if this RoleSet allows X11 Forwarding.
func (a *accessCheckerTAG) PermitX11Forwarding() bool {
	return false
}

// CertificateExtensions returns the list of extensions for each role in the RoleSet
func (a *accessCheckerTAG) CertificateExtensions() []*types.CertExtension {
	fmt.Printf("-------------------- CertExtensions %v %v\n", a.info.Traits, a.info.Roles)

	// In our case, roles are empty
	var certRoles types.CertRoles = types.CertRoles{Roles: []string{}}
	bytes, err := utils.FastMarshal(certRoles)
	if err != nil {
		fmt.Printf("Failed to marshal empty cert roles: %v\n", trace.BadParameter(err.Error()))
	}

	// Traits must be the same
	traitsJson, err := json.Marshal(a.info.Traits)
	if err != nil {
		fmt.Printf("Failed to marshal user traits: %v\n", err)
	}

	roles := types.CertExtension{Name: teleport.CertExtensionTeleportRoles, Value: string(bytes)}
	traits := types.CertExtension{Name: teleport.CertExtensionTeleportTraits, Value: string(traitsJson)}

	return []*types.CertExtension{&roles, &traits}
}

// GetAllLogins returns all valid unix logins for the AccessChecker.
func (a *accessCheckerTAG) GetAllLogins() []string {
	fmt.Println("-------------------- GetAllLogins")
	return []string{}
}

// HostUsers returns host user information matching a server or nil if
// a role disallows host user creation
func (a *accessCheckerTAG) HostUsers(types.Server) (*HostUsersInfo, error) {
	fmt.Println("-------------------- HostUsers")
	return nil, nil
}

func (a *accessCheckerTAG) GetAllowedLoginsForResource(resource AccessCheckable) ([]string, error) {
	// TODO implement me
	panic("implement me")
}
