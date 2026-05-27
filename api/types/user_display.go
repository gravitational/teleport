/*
Copyright 2026 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	"encoding/json"
	"strings"
)

const (
	oktaDisplayNameTrait = "okta/displayName"
	oktaGivenNameTrait   = "okta/givenName"
	oktaFamilyNameTrait  = "okta/familyName"
	oktaFirstNameTrait   = "okta/firstName"
	oktaLastNameTrait    = "okta/lastName"
	oktaEmailTrait       = "okta/email"

	scimAttrsLabel      = TeleportInternalLabelPrefix + "scim-attrs"
	scimDisplayNameAttr = "displayName"
	scimNameAttr        = "name"
	scimGivenNameAttr   = "givenName"
	scimFamilyNameAttr  = "familyName"

	displayNameTrait         = "displayName"
	nameTrait                = "name"
	givenNameTrait           = "givenName"
	familyNameTrait          = "familyName"
	firstNameTrait           = "firstName"
	lastNameTrait            = "lastName"
	emailTrait               = "email"
	givenNameSnakeCaseTrait  = "given_name"
	familyNameSnakeCaseTrait = "family_name"

	entraIDDisplayNameTrait = "http://schemas.microsoft.com/identity/claims/displayname"
	entraIDGivenNameTrait   = "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname"
	entraIDSurnameTrait     = "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/surname"
	entraIDEmailTrait       = "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress"
)

// UserDisplay contains display values derived from the user.
type UserDisplay struct {
	// Primary is a human-readable display name when distinct from username.
	Primary string
	// Secondary is supporting display context when distinct from username.
	Secondary string
}

// GetDisplay returns display values derived from the user.
func (u *UserV2) GetDisplay() UserDisplay {
	username := u.GetName()
	traits := u.GetTraits()

	display := UserDisplay{
		Primary:   displayPrimaryFromTraits(traits, username),
		Secondary: displaySecondaryFromTraits(traits, username),
	}
	if display.Primary != "" && display.Secondary != "" {
		return display
	}

	attrs := u.scimAttrs()
	if display.Primary == "" {
		display.Primary = displayPrimaryFromSCIMAttrs(attrs, username)
	}
	if display.Secondary == "" {
		display.Secondary = displaySecondaryFromSCIMAttrs(attrs, username)
	}
	return display
}

func displayPrimaryFromTraits(traits map[string][]string, username string) string {
	if display := displayPrimaryFromOktaTraits(traits, username); display != "" {
		return display
	}

	if display := displayPrimaryFromEntraIDTraits(traits, username); display != "" {
		return display
	}

	return displayPrimaryFromGenericTraits(traits, username)
}

func displaySecondaryFromTraits(traits map[string][]string, username string) string {
	if display := displaySecondaryFromOktaTraits(traits, username); display != "" {
		return display
	}

	if display := displaySecondaryFromEntraIDTraits(traits, username); display != "" {
		return display
	}

	return displaySecondaryFromGenericTraits(traits, username)
}

func displayPrimaryFromOktaTraits(traits map[string][]string, username string) string {
	return firstNonEmptyValueDifferentFromUsername(username,
		firstNonEmptyValueForKey(traits, oktaDisplayNameTrait),
		joinNonEmptyValuesForKeys(traits, oktaGivenNameTrait, oktaFamilyNameTrait),
		joinNonEmptyValuesForKeys(traits, oktaFirstNameTrait, oktaLastNameTrait),
	)
}

func displaySecondaryFromOktaTraits(traits map[string][]string, username string) string {
	return firstNonEmptyValueDifferentFromUsername(username, firstNonEmptyValueForKey(traits, oktaEmailTrait))
}

func displayPrimaryFromSCIMAttrs(attrs map[string]any, username string) string {
	return firstNonEmptyValueDifferentFromUsername(username,
		stringAtPath(attrs, scimDisplayNameAttr),
		joinNonEmptyStrings(
			stringAtPath(attrs, scimNameAttr, scimGivenNameAttr),
			stringAtPath(attrs, scimNameAttr, scimFamilyNameAttr),
		),
	)
}

func displaySecondaryFromSCIMAttrs(attrs map[string]any, username string) string {
	return firstNonEmptyValueDifferentFromUsername(username, scimEmailValue(attrs))
}

func displayPrimaryFromGenericTraits(traits map[string][]string, username string) string {
	return firstNonEmptyValueDifferentFromUsername(username,
		firstNonEmptyValueForKey(traits, displayNameTrait),
		firstNonEmptyValueForKey(traits, nameTrait),
		joinNonEmptyValuesForKeys(traits, givenNameTrait, familyNameTrait),
		joinNonEmptyValuesForKeys(traits, givenNameSnakeCaseTrait, familyNameSnakeCaseTrait),
		joinNonEmptyValuesForKeys(traits, firstNameTrait, lastNameTrait),
	)
}

func displaySecondaryFromGenericTraits(traits map[string][]string, username string) string {
	return firstNonEmptyValueDifferentFromUsername(username, firstNonEmptyValueForKey(traits, emailTrait))
}

func displayPrimaryFromEntraIDTraits(traits map[string][]string, username string) string {
	return firstNonEmptyValueDifferentFromUsername(username,
		firstNonEmptyValueForKey(traits, entraIDDisplayNameTrait),
		joinNonEmptyValuesForKeys(traits, entraIDGivenNameTrait, entraIDSurnameTrait),
	)
}

func displaySecondaryFromEntraIDTraits(traits map[string][]string, username string) string {
	return firstNonEmptyValueDifferentFromUsername(username, firstNonEmptyValueForKey(traits, entraIDEmailTrait))
}

func firstNonEmptyValueDifferentFromUsername(username string, values ...string) string {
	username = strings.TrimSpace(username)
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && value != username {
			return value
		}
	}
	return ""
}

func firstNonEmptyValueForKey(valuesByKey map[string][]string, key string) string {
	for _, value := range valuesByKey[key] {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func joinNonEmptyValuesForKeys(valuesByKey map[string][]string, keys ...string) string {
	values := make([]string, 0, len(keys))
	for _, key := range keys {
		values = append(values, firstNonEmptyValueForKey(valuesByKey, key))
	}
	return joinNonEmptyStrings(values...)
}

func joinNonEmptyStrings(values ...string) string {
	return strings.Join(strings.Fields(strings.Join(values, " ")), " ")
}

func (u *UserV2) scimAttrs() map[string]any {
	attrsJSON, _ := u.GetLabel(scimAttrsLabel)
	if strings.TrimSpace(attrsJSON) == "" {
		return nil
	}

	var attrs map[string]any
	if err := json.Unmarshal([]byte(attrsJSON), &attrs); err != nil {
		return nil
	}
	return attrs
}

// stringAtPath returns a trimmed string from a nested map path, or "" if any path segment is missing or non-string.
func stringAtPath(attrs map[string]any, path ...string) string {
	var current any = attrs
	for _, element := range path {
		attrMap, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = attrMap[element]
	}

	value, ok := current.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

// scimEmailValue returns the primary SCIM email value, or the first non-empty
// value if no primary email is set. See https://www.rfc-editor.org/info/rfc7643/#section-8.2.
func scimEmailValue(attrs map[string]any) string {
	emails, ok := attrs["emails"].([]any)
	if !ok {
		return ""
	}

	var first string
	for _, email := range emails {
		emailMap, ok := email.(map[string]any)
		if !ok {
			continue
		}

		value, _ := emailMap["value"].(string)
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if first == "" {
			first = value
		}
		primary, _ := emailMap["primary"].(bool)
		if primary {
			return value
		}
	}
	return first
}
