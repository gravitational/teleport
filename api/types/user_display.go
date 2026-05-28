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
	"cmp"
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

	sources := []displaySource{
		traitDisplaySource{
			traits: traits,
			primaryCandidates: [][]string{
				{oktaDisplayNameTrait},
				{oktaGivenNameTrait, oktaFamilyNameTrait},
				{oktaFirstNameTrait, oktaLastNameTrait},
			},
			secondaryCandidates: [][]string{
				{oktaEmailTrait},
			},
		},
		traitDisplaySource{
			traits: traits,
			primaryCandidates: [][]string{
				{entraIDDisplayNameTrait},
				{entraIDGivenNameTrait, entraIDSurnameTrait},
			},
			secondaryCandidates: [][]string{
				{entraIDEmailTrait},
			},
		},
		traitDisplaySource{
			traits: traits,
			primaryCandidates: [][]string{
				{displayNameTrait},
				{nameTrait},
				{givenNameTrait, familyNameTrait},
				{givenNameSnakeCaseTrait, familyNameSnakeCaseTrait},
				{firstNameTrait, lastNameTrait},
			},
			secondaryCandidates: [][]string{
				{emailTrait},
			},
		},
		scimDisplaySource{
			attrs: u.scimAttrs(),
			primaryCandidates: [][][]string{
				{{scimDisplayNameAttr}},
				{{scimNameAttr, scimGivenNameAttr}, {scimNameAttr, scimFamilyNameAttr}},
			},
			secondaryCandidates: []scimValueCandidate{
				scimEmailValue,
			},
		},
	}

	var display UserDisplay
	for _, source := range sources {
		if display.Primary == "" {
			display.Primary = source.primary(username)
		}
		if display.Secondary == "" {
			display.Secondary = source.secondary(username)
		}
		if display.Primary != "" && display.Secondary != "" {
			break
		}
	}
	return display
}

type displaySource interface {
	primary(username string) string
	secondary(username string) string
}

type traitDisplaySource struct {
	traits              map[string][]string
	primaryCandidates   [][]string
	secondaryCandidates [][]string
}

func (s traitDisplaySource) primary(username string) string {
	return displayValueFromTraitCandidates(s.traits, username, s.primaryCandidates)
}

func (s traitDisplaySource) secondary(username string) string {
	return displayValueFromTraitCandidates(s.traits, username, s.secondaryCandidates)
}

type scimDisplaySource struct {
	attrs               map[string]any
	primaryCandidates   [][][]string
	secondaryCandidates []scimValueCandidate
}

func (s scimDisplaySource) primary(username string) string {
	return displayValueFromSCIMAttrCandidates(s.attrs, username, s.primaryCandidates)
}

func (s scimDisplaySource) secondary(username string) string {
	return displayValueFromSCIMValueCandidates(s.attrs, username, s.secondaryCandidates)
}

type scimValueCandidate func(map[string]any) string

func displayValueFromTraitCandidates(traits map[string][]string, username string, candidates [][]string) string {
	values := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		values = append(values, joinNonEmptyValuesForKeys(traits, candidate...))
	}
	return cmp.Or(valuesDifferentFromUsername(username, values...)...)
}

func displayValueFromSCIMAttrCandidates(attrs map[string]any, username string, candidates [][][]string) string {
	values := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		parts := make([]string, 0, len(candidate))
		for _, path := range candidate {
			parts = append(parts, stringAtPath(attrs, path...))
		}
		values = append(values, joinNonEmptyStrings(parts...))
	}
	return cmp.Or(valuesDifferentFromUsername(username, values...)...)
}

func displayValueFromSCIMValueCandidates(attrs map[string]any, username string, candidates []scimValueCandidate) string {
	values := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		values = append(values, candidate(attrs))
	}
	return cmp.Or(valuesDifferentFromUsername(username, values...)...)
}

func valuesDifferentFromUsername(username string, values ...string) []string {
	username = strings.TrimSpace(username)
	candidates := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == username {
			value = ""
		}
		candidates = append(candidates, value)
	}
	return candidates
}

func trimmedValues(values ...string) []string {
	candidates := make([]string, 0, len(values))
	for _, value := range values {
		candidates = append(candidates, strings.TrimSpace(value))
	}
	return candidates
}

func joinNonEmptyValuesForKeys(valuesByKey map[string][]string, keys ...string) string {
	values := make([]string, 0, len(keys))
	for _, key := range keys {
		values = append(values, cmp.Or(trimmedValues(valuesByKey[key]...)...))
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
