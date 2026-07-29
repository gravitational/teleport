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

// Package replayfixture stores a beam's recorded activity -- its audit events
// and the app-session-chunk recordings they reference -- in a single JSON file,
// and serves it back as an audit source. It lets the beam replay pipeline be
// run offline against a real beam, away from the cluster the beam ran on:
// "tctl beams export" writes a fixture, and tests and the beam dev tools load
// one and feed it to the replay sink in place of a live audit log.
//
// Fixtures are exports of real sessions, so they carry recorded prompts,
// responses, and headers verbatim. They are meant to stay untracked local
// files, not test data committed to the repository.
package replayfixture

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
)

// JobParams is the beam metadata the replay pipeline needs alongside the
// recordings: enough to rebuild the replay Job that produced the artifact the
// beam was exported from.
type JobParams struct {
	// BeamID is the beam's UUID, which every recorded event is attributed to.
	BeamID string `json:"beam_id"`
	// Alias is the beam's human-readable name.
	Alias string `json:"alias,omitempty"`
	// Owner is the user the beam ran as.
	Owner string `json:"owner,omitempty"`
	// AppName is the published name of the inference endpoint app the beam
	// proxied its LLM traffic through.
	AppName string `json:"app_name,omitempty"`
	// LLMFormat is the wire format of the recorded LLM traffic ("openai" or
	// "anthropic"). It selects how bodies are decoded, so a fixture without it
	// segments into nothing; New infers it from the recordings when the caller
	// does not supply it.
	LLMFormat string `json:"llm_format,omitempty"`
	// CreatedAt and ExpiresAt bound the beam's lifetime. The replay sink derives
	// its audit search window from them.
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Fixture is a beam's exported activity. Events and Chunks hold audit events in
// their dynamic map form (the same representation the audit log backends store),
// which keeps the file readable with jq and decodable by any Teleport build that
// knows the event types.
type Fixture struct {
	// Job is the exported beam's metadata.
	Job JobParams `json:"job"`
	// Events are the audit events attributed to the beam, in the order the audit
	// log returned them. App-session-chunk events are included: they are what
	// names the recordings in Chunks.
	Events []events.EventFields `json:"events"`
	// Chunks maps an app-session-chunk recording ID to that recording's events.
	// One HTTP exchange can straddle two recordings, so a consumer has to read
	// all of them before assembling exchanges.
	Chunks map[string][]events.EventFields `json:"chunks"`
}

// New builds a fixture from a beam's audit events and the chunk recordings they
// reference. Events that cannot be converted to their dynamic form are reported
// rather than silently dropped, so an export never looks complete when it lost
// part of a session.
func New(job JobParams, beamEvents []apievents.AuditEvent, chunks map[string][]apievents.AuditEvent) (*Fixture, error) {
	fixtureEvents, err := toFields(beamEvents)
	if err != nil {
		return nil, trace.Wrap(err, "converting beam events")
	}
	fixtureChunks := make(map[string][]events.EventFields, len(chunks))
	for id, evs := range chunks {
		fields, err := toFields(evs)
		if err != nil {
			return nil, trace.Wrap(err, "converting chunk recording %q", id)
		}
		fixtureChunks[id] = fields
	}

	f := &Fixture{Job: job, Events: fixtureEvents, Chunks: fixtureChunks}
	if f.Job.LLMFormat == "" {
		f.Job.LLMFormat = inferLLMFormat(chunks)
	}
	return f, nil
}

func toFields(evs []apievents.AuditEvent) ([]events.EventFields, error) {
	if len(evs) == 0 {
		return nil, nil
	}
	out := make([]events.EventFields, 0, len(evs))
	for _, ev := range evs {
		fields, err := events.ToEventFields(ev)
		if err != nil {
			return nil, trace.Wrap(err, "converting %s event", ev.GetType())
		}
		out = append(out, fields)
	}
	return out, nil
}

// LoadFile reads a fixture written by WriteFile.
func LoadFile(path string) (*Fixture, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	var f Fixture
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, trace.Wrap(err, "parsing fixture %q", path)
	}
	return &f, nil
}

// WriteFile writes the fixture to path. The JSON is not indented: a fixture of
// a real beam runs to millions of base64 body bytes, where indentation costs
// size without making anything more readable.
func (f *Fixture) WriteFile(path string) error {
	data, err := json.Marshal(f)
	if err != nil {
		return trace.Wrap(err, "encoding fixture")
	}
	// 0600: a fixture carries a session's recorded prompts and responses.
	if err := os.WriteFile(path, data, 0600); err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

// AuditEvents decodes the beam's audit events back into their typed form.
func (f *Fixture) AuditEvents() ([]apievents.AuditEvent, error) {
	evs, err := events.FromEventFieldsSlice(f.Events)
	return evs, trace.Wrap(err)
}

// ChunkEvents decodes the events of the chunk recording named id. It reports
// NotFound if the fixture has no such recording, which distinguishes a chunk
// missing from the export from one that was recorded empty.
func (f *Fixture) ChunkEvents(id string) ([]apievents.AuditEvent, error) {
	fields, ok := f.Chunks[id]
	if !ok {
		return nil, trace.NotFound("fixture has no chunk recording %q", id)
	}
	evs, err := events.FromEventFieldsSlice(fields)
	return evs, trace.Wrap(err)
}

// inferLLMFormat guesses the beam's LLM wire format from the recorded requests.
// The replay manifest does not carry the format, so an export has only the
// traffic to go on: the request path identifies the API (Anthropic's Messages
// API versus OpenAI's Responses/Chat Completions), and the provider reported by
// the LLM request events is the fallback for a path that names neither. It
// returns "" when nothing in the export is recognizable, leaving the caller to
// pass the format in explicitly.
func inferLLMFormat(chunks map[string][]apievents.AuditEvent) string {
	var provider string
	for _, evs := range chunks {
		for _, ev := range evs {
			switch e := ev.(type) {
			case *apievents.AppSessionHTTPRequest:
				switch path := strings.ToLower(e.Url); {
				case strings.Contains(path, "/messages"):
					return types.LLMFormatAnthropic
				case strings.Contains(path, "/responses"), strings.Contains(path, "/chat/completions"):
					return types.LLMFormatOpenAI
				}
			case *apievents.AppSessionLLMRequest:
				if provider == "" {
					provider = e.Provider
				}
			}
		}
	}
	switch provider {
	case types.LLMProviderAnthropic:
		return types.LLMFormatAnthropic
	case types.LLMProviderOpenAI:
		return types.LLMFormatOpenAI
	default:
		// Bedrock serves both formats, so its provider says nothing about the
		// wire format.
		return ""
	}
}
