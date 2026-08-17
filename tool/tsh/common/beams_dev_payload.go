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
	"context"
	"embed"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/trace"
)

//go:embed beams_dev_payload/*.sh
var beamsDevPayloadFS embed.FS

const beamsDevRemoteMetaDir = "/home/beams/.beams-dev"

// beamsDevCrontab is installed on the beam so the payload runs with no laptop
// attached: the dead-man's WIP push every 5 minutes and the self-handoff
// check every 10. Each invocation is bounded by timeout so a single hung run
// (the scripts also bound their own network calls) can never wedge the beam's
// safety net for good.
const beamsDevCrontab = `*/5 * * * * timeout -k 30 240 /home/beams/.beams-dev/wip-push.sh
*/10 * * * * timeout -k 60 3000 /home/beams/.beams-dev/handoff.sh
`

// installBeamsDevPayload ships the beam-side scripts, their metadata, and the
// cron entries to the attached beam. Idempotent: re-running refreshes
// everything in place.
func installBeamsDevPayload(ctx context.Context, runner *beamRunner, ws *devWorkspace) error {
	entries, err := beamsDevPayloadFS.ReadDir("beams_dev_payload")
	if err != nil {
		return trace.Wrap(err)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "mkdir -p %s", beamsDevRemoteMetaDir)
	for _, entry := range entries {
		data, err := beamsDevPayloadFS.ReadFile("beams_dev_payload/" + entry.Name())
		if err != nil {
			return trace.Wrap(err)
		}
		encoded := base64.StdEncoding.EncodeToString(data)
		fmt.Fprintf(&sb, " && echo %s | base64 -d > %s/%s && chmod +x %s/%s",
			encoded, beamsDevRemoteMetaDir, entry.Name(), beamsDevRemoteMetaDir, entry.Name())
	}

	meta := fmt.Sprintf("WORKSPACE_ID=%s\nREPO_DIR=%s\nEXPIRES_AT=%d\nSELF_UUID=%s\nSELF_ALIAS=%s\n",
		ws.ID, ws.RemoteDir, ws.BeamExpires.Unix(), ws.BeamUUID, ws.BeamAlias)
	fmt.Fprintf(&sb, " && echo %s | base64 -d > %s/meta.env",
		base64.StdEncoding.EncodeToString([]byte(meta)), beamsDevRemoteMetaDir)
	fmt.Fprintf(&sb, " && echo %s | base64 -d > %s/crontab",
		base64.StdEncoding.EncodeToString([]byte(beamsDevCrontab)), beamsDevRemoteMetaDir)
	// The workspace marker is what lets a client rediscover which beam belongs
	// to which workspace after losing its record of a handoff.
	fmt.Fprintf(&sb, " && echo %s > %s/workspace", ws.ID, beamsDevRemoteMetaDir)

	// Cap Claude Code's output tokens under the LLM proxy's non-streaming
	// limit (lib/srv/app/llm/anthropic rejects max_tokens > 21000 when not
	// streaming; Claude Code's default is higher). Best-effort: needs sudo.
	fmt.Fprintf(&sb,
		" && (sudo -n sh -c 'grep -q CLAUDE_CODE_MAX_OUTPUT_TOKENS /etc/environment || echo CLAUDE_CODE_MAX_OUTPUT_TOKENS=21000 >> /etc/environment' 2>/dev/null || true)")

	// Install cron entries; fall back to a detached poller on cron-less
	// images. The `beams-dev-poller` $0 marker makes the poller findable so
	// idempotent re-installs never stack a second copy.
	fmt.Fprintf(&sb,
		" && if command -v crontab >/dev/null 2>&1; then crontab %s/crontab; elif ! pgrep -f beams-dev-poller >/dev/null 2>&1; then (setsid bash -c 'while true; do timeout -k 30 240 %s/wip-push.sh; timeout -k 60 3000 %s/handoff.sh; sleep 300; done' beams-dev-poller >/dev/null 2>&1 < /dev/null &) fi",
		beamsDevRemoteMetaDir, beamsDevRemoteMetaDir, beamsDevRemoteMetaDir)

	_, err = runner.output(ctx, sb.String())
	return trace.Wrap(err)
}

// readBeamsDevSuccessor checks a (still-alive) beam for a self-handoff pointer,
// returning the successor's UUID and alias if one exists.
func readBeamsDevSuccessor(ctx context.Context, runner *beamRunner) (uuid, alias string, ok bool) {
	out, err := runner.output(ctx, fmt.Sprintf("cat %s/successor 2>/dev/null || true", beamsDevRemoteMetaDir))
	if err != nil {
		return "", "", false
	}
	fields := strings.Fields(strings.TrimSpace(lastLine(out)))
	if len(fields) != 2 {
		return "", "", false
	}
	return fields[0], fields[1], true
}

// readBeamsDevWorkspaceMarker returns the workspace ID a beam claims to serve,
// or "" when the beam has no beams-dev payload installed.
func readBeamsDevWorkspaceMarker(ctx context.Context, runner *beamRunner) string {
	out, err := runner.output(ctx, fmt.Sprintf("cat %s/workspace 2>/dev/null || true", beamsDevRemoteMetaDir))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(lastLine(out))
}

// readBeamsDevBeamState probes a beam for both workspace-ownership files in a
// single round trip: the marker (fully seeded workspace) and the claim
// (reserved by an in-flight handoff whose seeding may not have finished).
func readBeamsDevBeamState(ctx context.Context, runner *beamRunner) (marker, claim string) {
	out, err := runner.output(ctx, fmt.Sprintf(
		`echo "beams-dev-marker=$(cat %s/workspace 2>/dev/null)"; echo "beams-dev-claim=$(cat %s/claim 2>/dev/null)"`,
		beamsDevRemoteMetaDir, beamsDevRemoteMetaDir))
	if err != nil {
		return "", ""
	}
	for _, line := range strings.Split(out, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "beams-dev-marker="); ok {
			marker = v
		}
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "beams-dev-claim="); ok {
			claim = v
		}
	}
	return marker, claim
}

// refreshBeamsDevExpiry updates the payload's recorded expiry (used after the
// client refreshes beam state, keeping the beam-side handoff trigger honest).
func refreshBeamsDevExpiry(ctx context.Context, runner *beamRunner, expires time.Time) error {
	_, err := runner.output(ctx, fmt.Sprintf(
		"[ -f %s/meta.env ] && sed -i 's/^EXPIRES_AT=.*/EXPIRES_AT=%d/' %s/meta.env || true",
		beamsDevRemoteMetaDir, expires.Unix(), beamsDevRemoteMetaDir))
	return trace.Wrap(err)
}
