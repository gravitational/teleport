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
	"slices"

	"github.com/google/uuid"
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

// recordingOwner is a single owner of a session recording: the user that
// should appear as the session's principal and the freshly-generated session
// ID assigned to this copy of the recording.
type recordingOwner struct {
	user        string
	sessionID   string
}

// bootstrapResult holds the output of buildBootstrapState.
type bootstrapResult struct {
	state       *stateConfig
	creds       map[string]*credentials
	userMapping map[string]string // canonical user key → generated name

	// recordingOwners maps the logical recording ID (the name of the .tar file
	// under testdata/recordings) to the list of owners that reference it. Each
	// owner gets its own fresh session ID so duplicates don't collide.
	recordingOwners map[string][]recordingOwner

	// recordingMapping is the inverse lookup for tests: it maps a generated
	// username to a record of `logicalID → generatedSessionID`. Written to
	// .auth/recording-mapping.json so the TS fixture can resolve IDs.
	recordingMapping map[string]map[string]string
}

// canonicalUserKey produces a deterministic string key for a scanned user definition.
// The same key format is computed on the TypeScript side to look up the generated name.
// Roles are represented as the name string or "@file:<filename>" for file roles, sorted.
// Traits are sorted by key, with values sorted within each key.
func canonicalUserKey(su scannedUser) string {
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
		Roles  []string            `json:"roles"`
		Traits map[string][]string `json:"traits,omitempty"`
	}

	kd := keyDef{Roles: roles}

	if len(su.traits) > 0 {
		kd.Traits = make(map[string][]string, len(su.traits))
		for k, v := range su.traits {
			sorted := slices.Clone(v)
			slices.Sort(sorted)
			kd.Traits[k] = sorted
		}
	}

	data, _ := json.Marshal(kd)

	return string(data)
}

// writeUserMapping writes the user mapping JSON file that the Playwright
// fixture reads at runtime to resolve generated usernames.
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

// buildBootstrapState builds the stateConfig and credential map from scanned users.
// For each user it generates a human-readable name, credentials, resolves role
// references (built-in name or file ref), and collects custom roles (deduplicated
// by filename). It returns the generated name of the loginAs user.
func buildBootstrapState(e2eDir string, scannedUsers []scannedUser) (*bootstrapResult, error) {
	state := &stateConfig{}
	creds := make(map[string]*credentials)
	nameGen := newHumanIDGenerator()
	userMapping := make(map[string]string)
	recordingOwners := make(map[string][]recordingOwner)
	recordingMapping := make(map[string]map[string]string)

	// Track custom role files already loaded to deduplicate.
	customRolesByFile := make(map[string]*customRole)

	for _, su := range scannedUsers {
		if len(su.roles) == 0 {
			return nil, fmt.Errorf("user declaration has no roles; declare at least one role in test.use()")
		}

		name := nameGen.Generate()
		userMapping[canonicalUserKey(su)] = name

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

		for _, rec := range su.recordings {
			sid := uuid.NewString()
			recordingOwners[rec] = append(recordingOwners[rec], recordingOwner{
				user:      name,
				sessionID: sid,
			})
			if recordingMapping[name] == nil {
				recordingMapping[name] = make(map[string]string)
			}
			recordingMapping[name][rec] = sid
		}
	}

	return &bootstrapResult{
		state:            state,
		creds:            creds,
		userMapping:      userMapping,
		recordingOwners:  recordingOwners,
		recordingMapping: recordingMapping,
	}, nil
}

// writeRecordingMapping writes the recording-mapping JSON file. The TS
// `recordings` fixture reads this to resolve a test's logical recording ID to
// the session ID that was actually seeded into the Teleport instance.
func writeRecordingMapping(path string, mapping map[string]map[string]string) error {
	data, err := json.MarshalIndent(mapping, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling recording mapping: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating recording mapping directory: %w", err)
	}

	return os.WriteFile(path, data, 0o644)
}

// writeCredentialsFile writes the user-credentials JSON file that Playwright
// reads at startup. Written to a file rather than passed via an environment
// variable so the payload doesn't grow unbounded with user count.
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
