// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package hostid

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/aws"
)

const (
	// FileName is the file name where the host UUID file is stored
	FileName = "host_uuid"
)

// GetPath returns the path to the host UUID file given the data directory.
func GetPath(dataDir string) string {
	return filepath.Join(dataDir, FileName)
}

// ReadFile reads host UUID from the file in the data dir
func ReadFile(dataDir string) (string, error) {
	out, err := utils.ReadPath(GetPath(dataDir))
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			// do not convert to system error as this loses the ability to compare that it is a permission error
			return "", trace.Wrap(err)
		}
		return "", trace.ConvertSystemError(err)
	}
	id := strings.TrimSpace(string(out))
	if id == "" {
		return "", trace.NotFound("host uuid is empty")
	}
	return id, nil
}

// Generate a new host UUID based on the join method.
// Uses the EC2 node ID for EC2 instances, and generates a new UUID for all other join methods.
func Generate(ctx context.Context, joinMethod types.JoinMethod) (string, error) {
	switch joinMethod {
	case types.JoinMethodToken,
		types.JoinMethodUnspecified,
		types.JoinMethodAzureDevops,
		types.JoinMethodIAM,
		types.JoinMethodCircleCI,
		types.JoinMethodKubernetes,
		types.JoinMethodGitHub,
		types.JoinMethodGitLab,
		types.JoinMethodAzure,
		types.JoinMethodGCP,
		types.JoinMethodTPM,
		types.JoinMethodTerraformCloud,
		types.JoinMethodOracle:
		// Checking error instead of the usual uuid.New() in case uuid generation
		// fails due to not enough randomness. It's been known to happen happen when
		// Teleport starts very early in the node initialization cycle and /dev/urandom
		// isn't ready yet.
		rawID, err := uuid.NewRandom()
		if err != nil {
			return "", trace.BadParameter("" +
				"Teleport failed to generate host UUID. " +
				"This may happen if randomness source is not fully initialized when the node is starting up. " +
				"Please try restarting Teleport again.")
		}
		return rawID.String(), nil
	case types.JoinMethodEC2:
		hostUUID, err := aws.GetEC2NodeID(ctx)
		if err != nil {
			return "", trace.Wrap(err)
		}
		return hostUUID, nil
	default:
		return "", trace.BadParameter("unknown join method %q", joinMethod)
	}
}
