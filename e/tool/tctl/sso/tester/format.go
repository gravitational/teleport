package tester

import (
	"fmt"
	"strings"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/tool/tctl/sso/tester"
)

func formatSSOWarnings(description string, info *types.SSOWarnings) string {
	if info == nil {
		return ""
	}

	if len(info.Warnings) > 0 {
		return fmt.Sprintf("%v: %v. Warnings:\n%v\n", description, info.Message, tester.Indent(strings.Join(info.Warnings, "\n"), 2))
	}

	return fmt.Sprintf("%v: %v\n", description, info.Message)
}
