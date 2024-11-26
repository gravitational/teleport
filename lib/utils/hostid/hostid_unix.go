//go:build !windows

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
	"errors"
	"io/fs"
	"time"

	"github.com/google/renameio/v2"
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/utils"
)

// WriteFile writes host UUID into a file
func WriteFile(dataDir string, id string) error {
	err := renameio.WriteFile(GetPath(dataDir), []byte(id), 0o400)
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			//do not convert to system error as this loses the ability to compare that it is a permission error
			return trace.Wrap(err)
		}
		return trace.ConvertSystemError(err)
	}
	return nil
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

	for i := 0; i < o.iterationLimit; i++ {
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
