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

// bootstrapUser represents a user to be bootstrapped into the Teleport state.
type bootstrapUser struct {
	Name                string
	Roles               []string
	Traits              map[string][]string
	PasswordHashBase64  string
	CredentialIDBase64  string
	PublicKeyCBORBase64 string
}

// customRole represents a custom role loaded from a YAML file.
type customRole struct {
	name string
	YAML string
}

// stateConfig holds the data needed to render the bootstrap state template.
type stateConfig struct {
	Users       []bootstrapUser
	CustomRoles []customRole
}

// credentialsJSON is the JSON-serializable shape for the E2E_USERS_JSON env var.
type credentialsJSON struct {
	Password             string `json:"password"`
	WebauthnPrivateKey   string `json:"webauthnPrivateKey"`
	WebauthnCredentialId string `json:"webauthnCredentialId"`
}

// readRoleFile reads a YAML role file from e2eDir/testdata/roles/<filename>,
// extracts the role name from metadata.name, and returns a customRole.
func readRoleFile(e2eDir, filename string) (*customRole, error) {
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

	return &customRole{
		name: match[1],
		YAML: yaml,
	}, nil
}

// buildBootstrapState builds the stateConfig and credential map from scanned users.
// For each user it generates credentials, resolves role references (built-in name
// or file ref), and collects custom roles (deduplicated by filename).
func buildBootstrapState(e2eDir string, scannedUsers []scannedUser) (*stateConfig, map[string]*credentials, error) {
	state := &stateConfig{}
	creds := make(map[string]*credentials)

	// Track custom role files already loaded to deduplicate.
	customRolesByFile := make(map[string]*customRole)

	for _, su := range scannedUsers {
		userCredentials, err := generateUserCredentials()
		if err != nil {
			return nil, nil, fmt.Errorf("generating credentials for %s: %w", su.name, err)
		}

		creds[su.name] = userCredentials

		bu := bootstrapUser{
			Name:                su.name,
			Traits:              map[string][]string{"logins": {"root"}},
			PasswordHashBase64:  userCredentials.passwordHashBase64,
			CredentialIDBase64:  userCredentials.credentialIDBase64,
			PublicKeyCBORBase64: userCredentials.publicKeyCBORBase64,
		}

		for _, role := range su.roles {
			if role.file != "" {
				cr, ok := customRolesByFile[role.file]
				if !ok {
					cr, err = readRoleFile(e2eDir, role.file)
					if err != nil {
						return nil, nil, fmt.Errorf("reading role for user %s: %w", su.name, err)
					}

					customRolesByFile[role.file] = cr
					state.CustomRoles = append(state.CustomRoles, *cr)
				}

				bu.Roles = append(bu.Roles, cr.name)
			} else {
				bu.Roles = append(bu.Roles, role.name)
			}
		}

		state.Users = append(state.Users, bu)
	}

	return state, creds, nil
}

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
