package main

import (
	"bytes"
	"context"

	"github.com/antithesishq/antithesis-sdk-go/assert"
	"github.com/google/uuid"
	workloadclient "github.com/gravitational/teleport/e/tests/antithesis/workloads/lib/client"
	"github.com/gravitational/trace"
)

func runDeniedSSHCommandProperty(ctx context.Context, params *TestCaseParams) error {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	clt, cleanup, err := workloadclient.NewTeleportClient(ctx, workloadclient.TeleportClientConfig{
		Identity:            params.Identity,
		Host:                params.Target.host(),
		Labels:              params.Target.labels(),
		PredicateExpression: params.Target.PredicateExpression,
		HostLogin:           params.HostUser,
		Stdout:              stdout,
		Stderr:              stderr,
	})
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

	// When no nodes are found the target resolution on SSH can return BadParameter no nodes.
	denied := trace.IsAccessDenied(err) || trace.IsConnectionProblem(err) ||
		(params.Target.usesResourceMatcher() && (trace.IsNotFound(err) || trace.IsBadParameter(err)))
	assert.AlwaysOrUnreachable(denied, "Denied identity cannot SSH to target", details)
	if err == nil {
		return trace.Errorf("running SSH command unexpectedly succeeded")
	}
	if !denied {
		return trace.Wrap(err, "running SSH command")
	}

	return nil
}
