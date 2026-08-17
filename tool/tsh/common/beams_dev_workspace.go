/*
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

package common

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/gravitational/trace"
)

// devWorkspace is the client-side record binding a local git checkout to the
// beam currently acting as its execution host. The local checkout is the
// durable half of the pair: the beam is expected to expire (24h TTL) and be
// replaced by a successor, at which point only BeamUUID/BeamAlias/BeamExpires
// change.
type devWorkspace struct {
	// ID is a stable identifier derived from the absolute local path.
	ID string `json:"id"`
	// LocalDir is the absolute path of the local git worktree root.
	LocalDir string `json:"local_dir"`
	// RepoName is the directory name used for the checkout on the beam,
	// i.e. the repo lives at /home/beams/<RepoName>.
	RepoName string `json:"repo_name"`
	// RemoteDir is the absolute path of the checkout on the beam.
	RemoteDir string `json:"remote_dir"`
	// Branch is the branch checked out at attach time.
	Branch string `json:"branch"`
	// OriginURL is the local checkout's remote.origin.url, if any. Used to
	// clone at datacenter speed on the beam instead of uploading the repo.
	OriginURL string `json:"origin_url,omitempty"`
	// BeamUUID/BeamAlias identify the currently attached beam.
	BeamUUID  string `json:"beam_uuid"`
	BeamAlias string `json:"beam_alias"`
	// BeamExpires is the attached beam's spec.expires, cached so the TTL
	// watcher doesn't need a round trip every tick.
	BeamExpires time.Time `json:"beam_expires"`
	// SetupScript is the repo-relative path of the provisioning script run
	// on fresh beams (empty when the repo has none).
	SetupScript string `json:"setup_script,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

const beamsDevRemoteHome = "/home/beams"

// beamsDevWorkspaceID derives the stable workspace ID for a local directory.
func beamsDevWorkspaceID(localDir string) string {
	sum := sha256.Sum256([]byte(localDir))
	return hex.EncodeToString(sum[:])[:12]
}

// beamsDevStateRoot returns the root directory for all beams-dev client state
// (~/.tsh/beams_dev), creating it if needed.
func beamsDevStateRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", trace.Wrap(err)
	}
	root := filepath.Join(home, ".tsh", "beams_dev")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", trace.Wrap(err)
	}
	return root, nil
}

// beamsDevWorkspaceStateDir returns the per-workspace state directory used for
// mirrored Claude transcripts, temp bundles, etc.
func beamsDevWorkspaceStateDir(id string) (string, error) {
	root, err := beamsDevStateRoot()
	if err != nil {
		return "", trace.Wrap(err)
	}
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", trace.Wrap(err)
	}
	return dir, nil
}

func beamsDevWorkspacePath(id string) (string, error) {
	root, err := beamsDevStateRoot()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return filepath.Join(root, id+".json"), nil
}

// loadDevWorkspace reads the workspace record for a local directory, returning
// (nil, nil) when none exists yet.
func loadDevWorkspace(localDir string) (*devWorkspace, error) {
	path, err := beamsDevWorkspacePath(beamsDevWorkspaceID(localDir))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var ws devWorkspace
	if err := json.Unmarshal(data, &ws); err != nil {
		return nil, trace.Wrap(err, "corrupt workspace record %s", path)
	}
	return &ws, nil
}

func saveDevWorkspace(ws *devWorkspace) error {
	ws.UpdatedAt = time.Now().UTC()
	path, err := beamsDevWorkspacePath(ws.ID)
	if err != nil {
		return trace.Wrap(err)
	}
	data, err := json.MarshalIndent(ws, "", "  ")
	if err != nil {
		return trace.Wrap(err)
	}
	// Write via a temp file + rename so a crash mid-write can't corrupt the
	// record that the resurrection path depends on.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(os.Rename(tmp, path))
}

func deleteDevWorkspace(id string) error {
	path, err := beamsDevWorkspacePath(id)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return trace.Wrap(err)
	}
	if dir, err := beamsDevWorkspaceStateDir(id); err == nil {
		_ = os.RemoveAll(dir)
	}
	return nil
}
