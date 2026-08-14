package accesslist

import (
	"github.com/gravitational/trace"
)

func validateMemberRequest(req memberGetter, opts memberOptions) error {
	// For backward compatibility reasons we can do this check only when this is a request for
	// a static Access List member. For the remaining requests spec.name will be always
	// overwritten with metadata.name.
	if opts.requireStatic {
		metadataName := req.GetMember().GetHeader().GetMetadata().GetName()
		specName := req.GetMember().GetSpec().GetName()
		if specName != "" && specName != metadataName {
			return trace.BadParameter(
				"The values of member.header.metadata.name (%q) and member.spec.name (%q) must match, unless member.spec.name is left empty. Tip: You can have multiple members with the same metadata.name as long as each of them has a different spec.access_list (i.e., they belong to different access lists",
				metadataName, specName,
			)
		}
	}
	if req.GetMember().GetSpec().GetAccessList() == "" {
		return trace.BadParameter("request's member.spec.access_list field is not set")
	}
	if req.GetMember().GetHeader().GetMetadata().GetName() == "" {
		return trace.BadParameter("request's member.header.metadata.name field is not set")
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
