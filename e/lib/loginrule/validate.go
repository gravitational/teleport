package loginrule

import (
	"github.com/gravitational/trace"

	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/api/types"
)

// Validate returns a non-nil error if the given login rule is misconfigured or
// contains an expression that fails to parse.
func Validate(rule *loginrulepb.LoginRule) error {
	switch {
	case !rule.HasMetadata():
		return trace.BadParameter("login rule resource must contain metadata")
	case rule.GetMetadata().Name == "":
		return trace.BadParameter("login rule resource must have non-empty metadata.name")
	case rule.GetVersion() != types.V1:
		return trace.BadParameter("unsupported login rule resource version %q, current supported version is %s", rule.GetVersion(), types.V1)
	case len(rule.GetTraitsMap()) > 0 && rule.GetTraitsExpression() != "":
		return trace.BadParameter("both traits_map and traits_expression are non-empty, exactly one must be set")
	case len(rule.GetTraitsMap()) == 0 && rule.GetTraitsExpression() == "":
		return trace.BadParameter("both traits_map and traits_expression are empty, exactly one must be set")
	}

	for trait, exprs := range rule.GetTraitsMap() {
		for _, e := range exprs.Values {
			_, err := parseExpr(e)
			if err != nil {
				return trace.Wrap(err, "failed to parse expression %q for trait %q", e, trait)
			}
		}
	}

	if len(rule.GetTraitsExpression()) > 0 {
		_, err := parseExpr(rule.GetTraitsExpression())
		if err != nil {
			return trace.Wrap(err, "failed to parse expression %q", rule.GetTraitsExpression())
		}
	}

	return nil
}
