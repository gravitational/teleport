package common

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	accessRoleContext   = "access-okta-acl-role"
	reviewerRoleContext = "reviewer-okta-acl-role"
)

// CreateOktaAccessRoleFriendlyName creates a friendly name for an Okta access role based on the
// Okta application Label and ID.
func CreateOktaAccessRoleFriendlyName(title string, id string) string {
	return oktaResourceFriendlyName(title, id, accessRoleContext)
}

// CreateOktaReviewerRoleFriendlyName creates a friendly name for an Okta reviewer role based on
// the Okta application Label and ID.
func CreateOktaReviewerRoleFriendlyName(title string, id string) string {
	return oktaResourceFriendlyName(title, id, reviewerRoleContext)
}

func oktaResourceFriendlyName(title string, id string, resourceContext string) string {
	if title == "" {
		return fmt.Sprintf("%s-%s", resourceContext, id)
	}
	return fmt.Sprintf("%s-%s-%s", normalizeOktaResourceName(title), resourceContext, id)
}

var allowCharacters = regexp.MustCompile(`^[0-9a-z\-@:]+$`)

func normalizeOktaResourceName(name string) string {
	name = strings.ToLower(name)

	var sb strings.Builder
	for _, r := range name {
		if allowCharacters.MatchString(string(r)) {
			sb.WriteRune(r)
			continue
		}
		// Replace disallowed characters with '_'
		if sb.Len() > 0 && sb.String()[sb.Len()-1] != '_' {
			sb.WriteRune('_')
		}
	}
	return strings.Trim(sb.String(), "_")
}
