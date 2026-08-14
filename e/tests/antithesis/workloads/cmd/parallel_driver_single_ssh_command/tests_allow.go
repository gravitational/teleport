package main

import (
	"bytes"
	"context"
	"strings"
	"time"

	"github.com/antithesishq/antithesis-sdk-go/assert"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
)

func runAllowedSSHCommandProperty(ctx context.Context, params *TestCaseParams) error {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	clt, cleanup, err := setupClient(ctx, params, stdout, stderr)
	if err != nil {
		return trace.Wrap(err)
	}
	defer cleanup()

	marker, err := uuid.NewV7()
	if err != nil {
		return trace.Wrap(err, "generating marker")
	}

	command := []string{"/bin/echo", marker.String()}
	start := time.Now().Add(-maxTolerableClockJitter)
	err = clt.SSH(ctx, command)
	// Padding to account for delay between session ending and the event being emitted.
	end := time.Now().Add(auditEventEmitDeadline + maxTolerableClockJitter)

	details := params.Details(map[string]any{
		"command": command,
		"error":   err,
		"stdout":  stdout.String(),
		"stderr":  stderr.String(),
	})

	// Hint to the fuzzer that we want successful SSH sessions to continue into
	// the output assertion when faults are not blocking the cluster.
	assert.Sometimes(err == nil, "Sometimes running an SSH command succeeds", details)
	if err != nil {
		return trace.Wrap(err, "running SSH command")
	}

	got := strings.TrimSpace(stdout.String())
	details["got"] = got
	details["want"] = marker.String()
	assert.AlwaysOrUnreachable(got == marker.String(), "SSH command returns expected output", details)
	if got != marker.String() {
		return trace.BadParameter("unexpected SSH command output")
	}

	return trace.Wrap(params.assertSSHCommandAuditEvents(ctx, marker.String(), start, end))
}
