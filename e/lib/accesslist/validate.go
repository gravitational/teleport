package accesslist

import (
	"github.com/gravitational/trace"
)

func validateMemberRequest(req memberGetter) error {
	if req.GetMember().GetSpec().GetAccessList() == "" {
		return trace.BadParameter("request's member.spec.access_list field is not set")
	}
	if req.GetMember().GetSpec().GetName() == "" {
		return trace.BadParameter("request's member.spec.name field is not set")
	}
	return nil
}

func validateMemberMetaRequest(req memberMetaGetter) error {
	if req.GetAccessList() == "" {
		return trace.BadParameter("request's access_list field is not set")
	}
	if req.GetMemberName() == "" {
		return trace.BadParameter("request's member_name field is not set")
	}
	return nil
}
