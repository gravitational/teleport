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
	"io"
	"os"
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v3"
)

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

// readRoleFile reads e2eDir/testdata/roles/<filename> and extracts metadata.name. Uses os.Root so filenames sourced from test code can't escape the roles directory.
func readRoleFile(e2eDir, filename string) (*customRole, error) {
	rolesDir := filepath.Join(e2eDir, "testdata", "roles")

	root, err := os.OpenRoot(rolesDir)
	if err != nil {
		return nil, fmt.Errorf("opening roles dir: %w", err)
	}
	defer root.Close()

	f, err := root.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("reading role file %s: %w", filename, err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("reading role file %s: %w", filename, err)
	}

	var meta struct {
		Metadata struct {
			Name string `yaml:"name"`
		} `yaml:"metadata"`
	}
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parsing role file %s: %w", filename, err)
	}

	if meta.Metadata.Name == "" {
		return nil, fmt.Errorf("role file %s: metadata.name not found", filename)
	}

	return &customRole{
		name: meta.Metadata.Name,
		YAML: string(data),
	}, nil
}

// bootstrapResult holds the output of buildBootstrapState.
type bootstrapResult struct {
	state       *stateConfig
	creds       map[string]*credentials
	userMapping map[string]string // canonical user key → generated name
}

// canonicalUserKey produces a deterministic key for a scanned user. The TS side computes the same format to look up generated names; both implementations must stay in lockstep.
func canonicalUserKey(su scannedUser) (string, error) {
	roles := make([]string, 0, len(su.roles))
	for _, r := range su.roles {
		if r.file != "" {
			roles = append(roles, "@file:"+r.file)
		} else {
			roles = append(roles, r.name)
		}
	}

	slices.Sort(roles)

	type keyDef struct {
		Index  *int                `json:"index,omitempty"`
		Roles  []string            `json:"roles"`
		Traits map[string][]string `json:"traits,omitempty"`
	}

	kd := keyDef{Index: su.arrayIdx, Roles: roles}

	if len(su.traits) > 0 {
		kd.Traits = make(map[string][]string, len(su.traits))
		for k, v := range su.traits {
			sorted := slices.Clone(v)
			slices.Sort(sorted)
			kd.Traits[k] = sorted
		}
	}

	data, err := json.Marshal(kd)
	if err != nil {
		return "", fmt.Errorf("marshaling canonical user key: %w", err)
	}

	return string(data), nil
}

// writeUserMapping writes the JSON file the Playwright fixture reads to resolve generated usernames.
func writeUserMapping(path string, mapping map[string]string) error {
	data, err := json.MarshalIndent(mapping, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling user mapping: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating user mapping directory: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// buildBootstrapState assigns names + credentials per scanned user, resolves role refs, and dedupes custom-role files.
func buildBootstrapState(e2eDir string, scannedUsers []scannedUser) (*bootstrapResult, error) {
	state := &stateConfig{}
	creds := make(map[string]*credentials)
	nameGen := newHumanIDGenerator()
	userMapping := make(map[string]string)

	// Track custom role files already loaded to deduplicate.
	customRolesByFile := make(map[string]*customRole)

	for _, su := range scannedUsers {
		if len(su.roles) == 0 {
			return nil, fmt.Errorf("user declaration has no roles; declare at least one role in test.use()")
		}

		name := nameGen.Generate()

		key, err := canonicalUserKey(su)
		if err != nil {
			return nil, err
		}

		userMapping[key] = name

		userCredentials, err := generateUserCredentials()
		if err != nil {
			return nil, fmt.Errorf("generating credentials for %s: %w", name, err)
		}

		creds[name] = userCredentials

		traits := su.traits
		if traits == nil {
			traits = map[string][]string{"logins": {"root"}}
		}

		bu := bootstrapUser{
			Name:                name,
			Traits:              traits,
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
						return nil, fmt.Errorf("reading role for user %s: %w", name, err)
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

	return &bootstrapResult{
		state:       state,
		creds:       creds,
		userMapping: userMapping,
	}, nil
}

// writeCredentialsFile writes the user-credentials JSON Playwright reads at startup. File-based (not env) so the payload doesn't grow unbounded with user count.
func writeCredentialsFile(path string, creds map[string]*credentials) error {
	m := make(map[string]credentialsJSON, len(creds))
	for name, c := range creds {
		m[name] = credentialsJSON{
			Password:             c.password,
			WebauthnPrivateKey:   c.privateKeyPKCS8Base64,
			WebauthnCredentialId: c.credentialIDBase64,
		}
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling credentials JSON: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating credentials directory: %w", err)
	}

	return os.WriteFile(path, data, 0o600)
}
