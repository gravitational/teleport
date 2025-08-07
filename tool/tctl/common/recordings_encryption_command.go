package common

import (
	"context"
	"fmt"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	recordingencryptionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
)

type recordingsEncryptionCommand struct {
	cmd         *kingpin.CmdClause
	rotateCmd   *kingpin.CmdClause
	statusCmd   *kingpin.CmdClause
	completeCmd *kingpin.CmdClause
	rollbackCmd *kingpin.CmdClause
}

func (c *recordingsEncryptionCommand) Initialize(recordingsCmd *kingpin.CmdClause) {
	c.cmd = recordingsCmd.Command("encryption", "Manage encryption properties of session recordings.")

	c.rotateCmd = c.cmd.Command("rotate", "Rotate encryption keys used for encrypting session recordings.")
	c.statusCmd = c.cmd.Command("status", "Show current rotation status")
	c.completeCmd = c.cmd.Command("complete", "Completes an in-progress encryption key rotation.")
	c.rollbackCmd = c.cmd.Command("rollback", "Rolls back an in-progress encryption key rotation.")
}

// TryRun attempts to run subcommands like "recordings encryption rotate".
func (c *recordingsEncryptionCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	var commandFunc func(ctx context.Context, client *authclient.Client) error
	switch cmd {
	case c.rotateCmd.FullCommand():
		commandFunc = c.Rotate
	case c.statusCmd.FullCommand():
		commandFunc = c.Status
	case c.completeCmd.FullCommand():
		commandFunc = c.Complete
	case c.rollbackCmd.FullCommand():
		commandFunc = c.Rollback
	default:
		return false, nil
	}
	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	err = commandFunc(ctx, client)
	closeFn(ctx)

	return true, trace.Wrap(err)
}

// Rotate handles
func (c *recordingsEncryptionCommand) Rotate(ctx context.Context, tc *authclient.Client) error {
	client := tc.RecordingEncryptionServiceClient()
	if _, err := client.RotateKey(ctx, &recordingencryptionv1.RotateKeyRequest{}); err != nil {
		return trace.Errorf("rotating key encryption keys: %v", err)
	}

	return nil
}

func (c *recordingsEncryptionCommand) Complete(ctx context.Context, tc *authclient.Client) error {
	client := tc.RecordingEncryptionServiceClient()
	if _, err := client.CompleteRotation(ctx, &recordingencryptionv1.CompleteRotationRequest{}); err != nil {
		return trace.Errorf("completing encryption key rotation: %v", err)
	}

	return nil
}

func (c *recordingsEncryptionCommand) Rollback(ctx context.Context, tc *authclient.Client) error {
	client := tc.RecordingEncryptionServiceClient()
	if _, err := client.RollbackRotation(ctx, &recordingencryptionv1.RollbackRotationRequest{}); err != nil {
		return trace.Errorf("rolling back encryption key rotation: %v", err)
	}

	return nil
}

func (c *recordingsEncryptionCommand) Status(ctx context.Context, tc *authclient.Client) error {
	client := tc.RecordingEncryptionServiceClient()
	res, err := client.GetRotationState(ctx, &recordingencryptionv1.GetRotationStateRequest{})
	if err != nil {
		return trace.Errorf("fetching encryption key status: %v", err)
	}

	rotationState := recordingencryptionv1.KeyPairState_KEY_PAIR_STATE_UNSPECIFIED
	t := asciitable.MakeTable([]string{"Key Pair Fingerprint", "State"})
	for _, pair := range res.KeyPairs {
		if pair.State == recordingencryptionv1.KeyPairState_KEY_PAIR_STATE_INACCESSIBLE {
			rotationState = pair.State
		}

		if pair.State == recordingencryptionv1.KeyPairState_KEY_PAIR_STATE_ROTATING {
			if rotationState != recordingencryptionv1.KeyPairState_KEY_PAIR_STATE_INACCESSIBLE {
				rotationState = pair.State
			}
		}

		t.AddRow([]string{pair.Fingerprint, c.getFriendlyStatusString(pair.State)})
	}

	switch rotationState {
	case recordingencryptionv1.KeyPairState_KEY_PAIR_STATE_INACCESSIBLE:
		fmt.Println("Rotation failed due to inaccessible key")
	case recordingencryptionv1.KeyPairState_KEY_PAIR_STATE_ROTATING:
		fmt.Println("Rotation in progress")

	}

	_, err = t.AsBuffer().WriteTo(os.Stdout)
	return trace.Wrap(err, "writing table")
}

func (c *recordingsEncryptionCommand) getFriendlyStatusString(state recordingencryptionv1.KeyPairState) string {
	switch state {
	case recordingencryptionv1.KeyPairState_KEY_PAIR_STATE_ROTATING:
		return "rotating"
	case recordingencryptionv1.KeyPairState_KEY_PAIR_STATE_ACTIVE:
		return "active"
	case recordingencryptionv1.KeyPairState_KEY_PAIR_STATE_INACCESSIBLE:
		return "inaccessible"
	default:
		return "unknown"
	}
}
