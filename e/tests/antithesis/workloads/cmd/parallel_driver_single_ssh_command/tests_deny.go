package main

import (
	"bytes"
	"context"

	"github.com/antithesishq/antithesis-sdk-go/assert"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
)

func runDeniedSSHCommandProperty(ctx context.Context, params *TestCaseParams) error {
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
	err = clt.SSH(ctx, command)

	details := params.Details(map[string]any{
		"command": command,
		"error":   err,
		"stdout":  stdout.String(),
		"stderr":  stderr.String(),
	})
	denied := trace.IsAccessDenied(err) || trace.IsConnectionProblem(err) ||
		(params.Target.usesResourceMatcher() && trace.IsNotFound(err))
	assert.AlwaysOrUnreachable(denied, "Denied identity cannot SSH to target", details)
	if err == nil {
		return trace.Errorf("running SSH command unexpectedly succeeded")
	}
	if !denied {
		return trace.Wrap(err, "running SSH command")
	}

	return nil
}
