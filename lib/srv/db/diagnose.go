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

package db

import (
	"context"
	"time"

	"github.com/gravitational/oxy/ratelimit"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

// diagnoseTask defines a task for diagnosing a database error.
type diagnoseTask struct {
	err      error
	database types.Database
}

// startDiagnoseRoutine starts a goroutine to process database errors for
// diagnosis.
func (s *Server) startDiagnoseRoutine(ctx context.Context) error {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return

			case task := <-s.dignoseCh:
				// Check if this database is still being proxied.
				proxied, found := s.getProxiedDatabase(task.database.GetName())
				if !found || proxied.GetURI() != task.database.GetURI() {
					break
				}

				s.processDiangoseTask(ctx, task)
				if s.cfg.OnDiagnose != nil {
					s.cfg.OnDiagnose(task.database, task.err)
				}
			}
		}
	}()
	return nil
}

// addDiagnoseTask adds a database with error for diagnosis.
func (s *Server) addDiagnoseTask(database types.Database, err error) error {
	if !common.IsIAMAuthError(err) {
		return nil
	}

	// Rate limit the diagnosis per database to avoid repetition.
	rates := ratelimit.NewRateSet()
	if err := rates.Add(defaultDiagnoseRatePeriod, 1, 1); err != nil {
		return trace.Wrap(err)
	}

	token := database.GetName() + database.GetURI()
	if err := s.cfg.Limiter.RegisterRequestWithCustomRate(token, rates); err != nil {
		return trace.LimitExceeded(err.Error())
	}

	select {
	case s.dignoseCh <- diagnoseTask{
		err:      err,
		database: database,
	}:
	default:
		return trace.LimitExceeded("Failed to queue database %v for diagnose.", database.GetName())
	}
	return nil
}

// processDiangoseTask diagnose a database error.
func (s *Server) processDiangoseTask(ctx context.Context, task diagnoseTask) {
	s.log.Debugf("Diagnosing database %v with error: %v", task.database, task.err)
	switch {
	case common.IsIAMAuthError(task.err):
		if err := s.cfg.CloudMeta.Update(ctx, task.database); err != nil {
			s.log.Warnf("Failed to fetch cloud metadata for %v: %v.", task.database, err)
		}
		if err := s.cfg.CloudIAM.Setup(ctx, task.database); err != nil {
			s.log.Warnf("Failed to auto-configure IAM for %v: %v.", task.database, err)
		}
	}
}

const (
	// defaultDiagnoseRatePeriod is the default rate limiting period per
	// database diagnosis.
	defaultDiagnoseRatePeriod = 10 * time.Minute

	// defaultDiagnoseChannelSize is the default size of diagnose channel.
	defaultDiagnoseChannelSize = 100
)
