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
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
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
		types.JoinMethodBitbucket,
		types.JoinMethodBoundKeypair,
		types.JoinMethodIAM,
		types.JoinMethodCircleCI,
		types.JoinMethodKubernetes,
		types.JoinMethodGenericOIDC,
		types.JoinMethodGitHub,
		types.JoinMethodGitLab,
		types.JoinMethodAzure,
		types.JoinMethodGCP,
		types.JoinMethodTPM,
		types.JoinMethodSpacelift,
		types.JoinMethodTerraformCloud,
		types.JoinMethodOracle,
		types.JoinMethodEnv0:
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

type options struct {
	retryConfig    retryutils.RetryV2Config
	iterationLimit int
}

// WithBackoff overrides the default backoff configuration of
// [ReadOrCreateFile].
func WithBackoff(cfg retryutils.RetryV2Config) func(*options) {
	return func(o *options) {
		o.retryConfig = cfg
	}
}

// WithIterationLimit overrides the default number of time
// [ReadOrCreateFile] will attempt to produce a hostid.
func WithIterationLimit(limit int) func(*options) {
	return func(o *options) {
		o.iterationLimit = limit
	}
}

// ReadOrCreateFile looks for a hostid file in the data dir. If present,
// returns the UUID from it, otherwise generates one.
func ReadOrCreateFile(dataDir string, opts ...func(*options)) (string, error) {
	o := options{
		retryConfig: retryutils.RetryV2Config{
			First:  100 * time.Millisecond,
			Driver: retryutils.NewLinearDriver(100 * time.Millisecond),
			Max:    time.Second,
			Jitter: retryutils.FullJitter,
		},
		iterationLimit: 3,
	}

	for _, opt := range opts {
		opt(&o)
	}

	hostUUIDFileLock := GetPath(dataDir) + ".lock"

	backoff, err := retryutils.NewRetryV2(o.retryConfig)
	if err != nil {
		return "", trace.Wrap(err)
	}

	for range o.iterationLimit {
		if read, err := ReadFile(dataDir); err == nil {
			return read, nil
		} else if !trace.IsNotFound(err) {
			return "", trace.Wrap(err)
		}

		// Checking error instead of the usual uuid.New() in case uuid generation
		// fails due to not enough randomness. It's been known to happen when
		// Teleport starts very early in the node initialization cycle and /dev/urandom
		// isn't ready yet.
		rawID, err := uuid.NewRandom()
		if err != nil {
			return "", trace.BadParameter("" +
				"Teleport failed to generate host UUID. " +
				"This may happen if randomness source is not fully initialized when the node is starting up. " +
				"Please try restarting Teleport again.")
		}

		writeFile := func(potentialID string) (string, error) {
			unlock, err := utils.FSTryWriteLock(hostUUIDFileLock)
			if err != nil {
				return "", trace.Wrap(err)
			}
			defer unlock()

			if read, err := ReadFile(dataDir); err == nil {
				return read, nil
			} else if !trace.IsNotFound(err) {
				return "", trace.Wrap(err)
			}

			if err := WriteFile(dataDir, potentialID); err != nil {
				return "", trace.Wrap(err)
			}

			return potentialID, nil
		}

		id, err := writeFile(rawID.String())
		if err != nil {
			if errors.Is(err, utils.ErrUnsuccessfulLockTry) {
				backoff.Inc()
				<-backoff.After()
				continue
			}

			return "", trace.Wrap(err)
		}
		backoff.Reset()

		return id, nil
	}

	return "", trace.LimitExceeded("failed to obtain host uuid")
}
