package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/google/uuid"
	"go.yaml.in/yaml/v3"
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
	ClientIP             string `json:"clientIp"`
}

// readRoleFile reads testdata/roles/<filename> from the first directory that has it, so an
// enterprise suite can carry its own roles without copying the shared ones. It extracts
// metadata.name. Uses os.Root so filenames sourced from test code can't escape
// the roles directory.
func readRoleFile(dirs []string, filename string) (*customRole, error) {
	var root *os.Root
	var f *os.File
	for _, dir := range dirs {
		r, err := os.OpenRoot(filepath.Join(dir, "testdata", "roles"))
		if err != nil {
			continue
		}

		if opened, err := r.Open(filename); err == nil {
			root, f = r, opened
			break
		}

		r.Close()
	}

	if f == nil {
		return nil, fmt.Errorf("reading role file %s: not found in %v", filename, dirs)
	}
	defer root.Close()
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

// recordingOwner is a single owner of a session recording: the user that
// should appear as the session's principal and the freshly-generated session
// ID assigned to this copy of the recording.
type recordingOwner struct {
	user      string
	sessionID string
}

type recordingOwners map[string][]recordingOwner

// bootstrapResult holds the output of buildBootstrapState.
type bootstrapResult struct {
	state       *stateConfig
	creds       map[string]*credentials
	userMapping map[string]string // canonical user key → generated name

	// recordingOwners maps the logical recording ID (the name of the .tar file
	// under testdata/recordings) to the list of owners that reference it. Each
	// owner gets its own fresh session ID so duplicates don't collide.
	recordingOwners recordingOwners

	// recordingMapping is the inverse lookup for tests: it maps a generated
	// username to a record of `logicalID → generatedSessionID`. Written to
	// .auth/recording-mapping.json so the TS fixture can resolve IDs.
	recordingMapping map[string]map[string]string
}

// canonicalUserKey produces a deterministic key for a scanned user. The TS
// side computes the same format to look up generated names; both
// implementations must stay in lockstep.
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
		Default bool                `json:"default,omitempty"`
		Source  string              `json:"source,omitempty"`
		Index   *int                `json:"index,omitempty"`
		Roles   []string            `json:"roles"`
		Traits  map[string][]string `json:"traits,omitempty"`
	}

	kd := keyDef{
		Default: su.isDefault,
		Source:  su.sourceFile,
		Index:   su.arrayIdx,
		Roles:   roles,
	}

	if len(su.traits) > 0 {
		traits := make(map[string][]string, len(su.traits))
		for k, v := range su.traits {
			if len(v) == 0 {
				continue
			}
			sorted := slices.Clone(v)
			slices.Sort(sorted)
			traits[k] = sorted
		}
		if len(traits) > 0 {
			kd.Traits = traits
		}
	}

	data, err := json.Marshal(kd)
	if err != nil {
		return "", fmt.Errorf("marshaling canonical user key: %w", err)
	}

	return string(data), nil
}

// writeUserMapping writes the JSON file the Playwright fixture reads to
// resolve generated usernames.
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

// buildBootstrapState assigns names + credentials per scanned user, resolves
// role refs, and dedupes custom-role files. Scanned users that share a
// canonical key are aggregated into a single bootstrap account whose
// recordings are the deduped union of every contributing declaration.
func buildBootstrapState(roleDirs []string, scannedUsers []scannedUser) (*bootstrapResult, error) {
	type userGroup struct {
		key string
		su  scannedUser
	}

	groupByKey := make(map[string]*userGroup)
	var groups []*userGroup
	for _, su := range scannedUsers {
		if len(su.roles) == 0 {
			return nil, fmt.Errorf("user declaration has no roles; declare at least one role in test.use()")
		}

		key, err := canonicalUserKey(su)
		if err != nil {
			return nil, err
		}

		if g, ok := groupByKey[key]; ok {
			for _, rec := range su.recordings {
				if !slices.Contains(g.su.recordings, rec) {
					g.su.recordings = append(g.su.recordings, rec)
				}
			}
			if su.loginAs {
				g.su.loginAs = true
			}
			continue
		}

		merged := su
		merged.recordings = slices.Clone(su.recordings)
		g := &userGroup{key: key, su: merged}
		groupByKey[key] = g
		groups = append(groups, g)
	}

	state := &stateConfig{}
	creds := make(map[string]*credentials)
	nameGen := newHumanIDGenerator()
	userMapping := make(map[string]string)
	recordingOwners := make(map[string][]recordingOwner)
	recordingMapping := make(map[string]map[string]string)

	// Track custom role files already loaded to deduplicate.
	customRolesByFile := make(map[string]*customRole)

	for i, g := range groups {
		su := g.su
		name := nameGen.Generate()
		userMapping[g.key] = name

		userCredentials, err := generateUserCredentials()
		if err != nil {
			return nil, fmt.Errorf("generating credentials for %s: %w", name, err)
		}

		userCredentials.clientIP = assignClientIP(i)

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
					cr, err = readRoleFile(roleDirs, role.file)
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

// writeRecordingMapping writes the recording-mapping JSON the TS `recordings`
// fixture reads to resolve a test's logical recording ID to the seeded session
// ID.
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

// writeCredentialsFile writes the user-credentials JSON Playwright reads at
// startup. File-based (not env) so the payload doesn't grow unbounded with
// user count.
func writeCredentialsFile(path string, creds map[string]*credentials) error {
	m := make(map[string]credentialsJSON, len(creds))
	for name, c := range creds {
		m[name] = credentialsJSON{
			Password:             c.password,
			WebauthnPrivateKey:   c.privateKeyPKCS8Base64,
			WebauthnCredentialId: c.credentialIDBase64,
			ClientIP:             c.clientIP,
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
