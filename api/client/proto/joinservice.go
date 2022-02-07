package proto

import "github.com/gravitational/trace"

func (r *RegisterUsingIAMMethodRequest) CheckAndSetDefaults() error {
	if len(r.STSIdentityRequest) == 0 {
		return trace.BadParameter("missing parameter STSIdentityRequest")
	}
	return trace.Wrap(r.RegisterUsingTokenRequest.CheckAndSetDefaults())
}
