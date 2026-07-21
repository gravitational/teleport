// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package llm

import (
	"log/slog"
	"net/http"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/httplib/httprecorder"
	"github.com/gravitational/teleport/lib/srv/app/common"
)

// BeamLLMRecordingEnabled reports whether LLM request/response traffic should
// be recorded for replay. It is only enabled when the app service is running
// inside a beam ([teleport.BeamsRuntimeEnvVar]) AND LLM recording has been explicitly
// turned on ([teleport.BeamsLLMRecordingEnvVar]), so ordinary app services never record
// request/response bodies. The environment is read via getenv (os.Getenv in
// production) so tests can inject values without mutating the process
// environment. Callers resolve this once and pass the result through
// [HandlerConfig.LLMRecordingEnabled].
func BeamLLMRecordingEnabled(getenv func(string) string) bool {
	inBeam, _ := apiutils.ParseBool(getenv(teleport.BeamsRuntimeEnvVar))
	//recording, _ := apiutils.ParseBool(getenv(teleport.BeamsLLMRecordingEnvVar))
	return inBeam && true
}

// maybeRecordSessionExchange serves r through next, recording the raw HTTP exchange
// (request/response envelopes and body chunks) into the session recording when
// beam LLM recording is enabled and the session has a chunk recorder.
// Recording is scoped to the LLM handler; no other app handler records traffic.
//
// Recording is fail-closed: if the initial request-metadata event cannot be
// recorded, next is not invoked and the error is returned so the caller can
// fail the request instead of proxying unrecorded traffic. Once the exchange
// has started, a failure finalizing the recording is logged rather than
// surfaced, since the response has already been sent.
func maybeRecordSessionExchange(log *slog.Logger, enabled bool, w http.ResponseWriter, r *http.Request, next http.Handler) error {
	if !enabled {
		next.ServeHTTP(w, r)
		return nil
	}

	sessionCtx, err := common.GetSessionContext(r)
	if err != nil {
		return trace.Wrap(err)
	}
	// The session chunk recorder is reached via the audit logger. Handlers
	// without a chunk recorder (e.g. TCP apps) have no audit here, in which
	// case recording is skipped rather than treated as an error.
	if sessionCtx.Audit == nil {
		next.ServeHTTP(w, r)
		return nil
	}
	recorder := sessionCtx.Audit.Recorder()
	if recorder == nil {
		next.ServeHTTP(w, r)
		return nil
	}

	// Record the LLM wire format and provider on the request event so consumers
	// (e.g. beam replay/summarization) don't have to infer them from the
	// endpoint. Empty when the app is not an LLM proxy.
	var format, provider string
	if llm := sessionCtx.App.GetLLM(); llm != nil {
		format = llm.Format
		provider = llm.Provider
	}

	recReq, recRW, err := httprecorder.New(httprecorder.Config{
		Request:        r,
		ResponseWriter: w,
		Recorder:       recorder,
		AppMetadata:    *common.MakeAppMetadata(sessionCtx.App),
		UserMetadata:   sessionCtx.Identity.GetUserMetadata(),
		Format:         format,
		Provider:       provider,
		Logger:         log,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		if finishErr := recRW.Finish(); finishErr != nil {
			log.ErrorContext(r.Context(), "failed to finalize LLM session recording", "error", finishErr)
		}
	}()

	next.ServeHTTP(recRW, recReq)

	return nil
}
