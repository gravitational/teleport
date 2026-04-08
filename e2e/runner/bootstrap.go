/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// metadataNameRe extracts the role name from a YAML file's metadata.name field.
var metadataNameRe = regexp.MustCompile(`(?m)^metadata:\s*\n\s+name:\s*(\S+)`)

// BootstrapUser represents a user to be bootstrapped into the Teleport state.
type BootstrapUser struct {
	Name                string
	Roles               []string
	Traits              map[string][]string
	PasswordHashBase64  string
	CredentialIDBase64  string
	PublicKeyCBORBase64 string
}

// CustomRole represents a custom role loaded from a YAML file.
type CustomRole struct {
	Name string
	YAML string
}

// StateConfig holds the data needed to render the bootstrap state template.
type StateConfig struct {
	Users       []BootstrapUser
	CustomRoles []CustomRole
}

// credentialsJSON is the JSON-serializable shape for the E2E_USERS_JSON env var.
type credentialsJSON struct {
	Password             string `json:"password"`
	WebauthnPrivateKey   string `json:"webauthnPrivateKey"`
	WebauthnCredentialId string `json:"webauthnCredentialId"`
}

// readRoleFile reads a YAML role file from e2eDir/testdata/roles/<filename>,
// extracts the role name from metadata.name, and returns a CustomRole.
func readRoleFile(e2eDir, filename string) (*CustomRole, error) {
	path := filepath.Join(e2eDir, "testdata", "roles", filename)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading role file %s: %w", filename, err)
	}

	yaml := string(data)

	match := metadataNameRe.FindStringSubmatch(yaml)
	if match == nil {
		return nil, fmt.Errorf("role file %s: metadata.name not found", filename)
	}

	return &CustomRole{
		Name: match[1],
		YAML: yaml,
	}, nil
}

// buildBootstrapState builds the StateConfig and credential map from scanned users.
// For each user it generates credentials, resolves role references (built-in name
// or file ref), and collects custom roles (deduplicated by filename).
func buildBootstrapState(e2eDir string, scannedUsers []ScannedUser) (*StateConfig, map[string]*credentials, error) {
	state := &StateConfig{}
	creds := make(map[string]*credentials)

	// Track custom role files already loaded to deduplicate.
	customRolesByFile := make(map[string]*CustomRole)

	for _, su := range scannedUsers {
		userCreds, err := generateUserCredentials()
		if err != nil {
			return nil, nil, fmt.Errorf("generating credentials for %s: %w", su.Name, err)
		}

		creds[su.Name] = userCreds

		bu := BootstrapUser{
			Name:                su.Name,
			Traits:              map[string][]string{"logins": {"root"}},
			PasswordHashBase64:  userCreds.passwordHashBase64,
			CredentialIDBase64:  userCreds.credentialIDBase64,
			PublicKeyCBORBase64: userCreds.publicKeyCBORBase64,
		}

		for _, role := range su.Roles {
			if role.File != "" {
				cr, ok := customRolesByFile[role.File]
				if !ok {
					cr, err = readRoleFile(e2eDir, role.File)
					if err != nil {
						return nil, nil, fmt.Errorf("reading role for user %s: %w", su.Name, err)
					}

					customRolesByFile[role.File] = cr
				}

				bu.Roles = append(bu.Roles, cr.Name)
			} else {
				bu.Roles = append(bu.Roles, role.Name)
			}
		}

		state.Users = append(state.Users, bu)
	}

	// Collect custom roles in a stable order based on first appearance.
	seen := make(map[string]bool)
	for _, su := range scannedUsers {
		for _, role := range su.Roles {
			if role.File != "" && !seen[role.File] {
				seen[role.File] = true
				state.CustomRoles = append(state.CustomRoles, *customRolesByFile[role.File])
			}
		}
	}

	return state, creds, nil
}

// marshalCredentialsJSON converts a credential map to a JSON string
// suitable for the E2E_USERS_JSON environment variable.
func marshalCredentialsJSON(creds map[string]*credentials) (string, error) {
	m := make(map[string]credentialsJSON, len(creds))
	for name, c := range creds {
		m[name] = credentialsJSON{
			Password:             c.password,
			WebauthnPrivateKey:   c.privateKeyPKCS8Base64,
			WebauthnCredentialId: c.credentialIDBase64,
		}
	}

	data, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("marshaling credentials JSON: %w", err)
	}

	return string(data), nil
}
