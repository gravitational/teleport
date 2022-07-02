/*
Copyright 2022 Gravitational, Inc.

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

package reporting

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/user"
	"path/filepath"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"

	"github.com/gravitational/reporting/types"
	"github.com/gravitational/trace"
)

// GetLicenseStatus gets the license status from the auth server.
func GetLicenseStatus(ctx context.Context, c *auth.Client) (*types.Heartbeat, error) {
	out, err := c.Get(ctx, c.Endpoint("license", "status"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	heartbeat, err := types.UnmarshalHeartbeat(out.Bytes())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return heartbeat, nil
}

// GetLicenseWarnings returns a list of license out of compliance warnings.
func GetLicenseWarnings(dir, profile string) (warnings []string, err error) {
	statusFile, err := licenseStatusFile(dir, profile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer statusFile.Close()
	warnings, err = getLicenseWarnings(statusFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return warnings, nil
}

func getLicenseWarnings(r io.Reader) (warnings []string, err error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	status, err := types.UnmarshalHeartbeat(b)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	warnings = make([]string, 0)
	for _, notification := range status.Spec.Notifications {
		if notification.Type == "licenseExpired" {
			warnings = append(warnings, notification.Text)
		}
	}
	return warnings, nil
}

// WriteLicenseStatus writes the license status.
func WriteLicenseStatus(dir, profile string, status *types.Heartbeat) error {
	statusFile, err := licenseStatusFile(dir, profile)
	if err != nil {
		return trace.Wrap(err)
	}
	defer statusFile.Close()
	if err := writeLicenseStatus(statusFile, status); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func writeLicenseStatus(w io.Writer, status *types.Heartbeat) error {
	b, err := types.MarshalHeartbeat(*status)
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := w.Write(b); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func licenseStatusFile(dir, profile string) (statusFile *os.File, err error) {
	if dir == "" {
		dir = defaultLicenseStatusDir()
	}
	if err := os.MkdirAll(dir, os.ModeDir|teleport.PrivateDirMode); err != nil {
		return nil, trace.Wrap(err)
	}
	path := filepath.Join(dir, fmt.Sprintf("%s.yaml", profile))
	statusFile, err = os.OpenFile(path, os.O_RDWR|os.O_CREATE, teleport.FileMaskOwnerOnly)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return statusFile, nil
}

func defaultLicenseStatusDir() string {
	home := os.TempDir()
	if u, err := user.Current(); err == nil && u.HomeDir != "" {
		home = u.HomeDir
	}
	return filepath.Join(home, tshDir, licenseStatusDir)
}

const tshDir = ".tsh"
const licenseStatusDir = "license-status"
