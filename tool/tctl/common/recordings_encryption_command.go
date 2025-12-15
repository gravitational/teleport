// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package common

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	recordingencryptionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
)

type recordingsEncryptionCommand struct {
	// cmd implements the "tctl recordings encryptino" parent command
	cmd *kingpin.CmdClause

	// rotateCmd implements the "tctl recordings encryption rotate" subcommand.
	rotateCmd *kingpin.CmdClause

	// statusCmd implements the "tctl recordings encryption status" subcommand.
	statusCmd *kingpin.CmdClause

	// completeCmd implements the "tctl recordings encryption complete" subcommand.
	completeCmd *kingpin.CmdClause

	// rollbackCmd implements the "tctl recordings encryption rollback" subcommand.
	rollbackCmd *kingpin.CmdClause

	// format is the output format of statusCmd (text, json, or yaml)
	format string

	// stdout allows for redirecting command output. Useful for tests.
	stdout io.Writer
}

// Initialize allows recordingsEncryptionCommand to plug itself into the CLI parser.
func (c *recordingsEncryptionCommand) Initialize(recordingsCmd *kingpin.CmdClause, stdout io.Writer) {
	c.cmd = recordingsCmd.Command("encryption", "Manage encryption properties of session recordings.")

	c.rotateCmd = c.cmd.Command("rotate", "Rotate encryption keys used for encrypting session recordings.")
	c.statusCmd = c.cmd.Command("status", "Show current rotation status.")
	c.statusCmd.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)+". Defaults to 'text'.").Default(teleport.Text).StringVar(&c.format)
	c.completeCmd = c.cmd.Command("complete-rotation", "Completes an in-progress encryption key rotation.")
	c.rollbackCmd = c.cmd.Command("rollback-rotation", "Rolls back an in-progress encryption key rotation.")
	if stdout == nil {
		c.stdout = os.Stdout
	}
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

// Rotate initiates a key rotation. It should fail if a key rotation is already
// in progress.
func (c *recordingsEncryptionCommand) Rotate(ctx context.Context, tc *authclient.Client) error {
	client := tc.RecordingEncryptionServiceClient()
	if _, err := client.RotateKey(ctx, &recordingencryptionv1.RotateKeyRequest{}); err != nil {
		return trace.Errorf("rotating key encryption keys: %v", err)
	}
	fmt.Fprintln(c.stdout, "Rotation started")
	return nil
}

// Complete an in progress key rotation. It should fail if any key is marked
// 'inaccessible'.
func (c *recordingsEncryptionCommand) Complete(ctx context.Context, tc *authclient.Client) error {
	client := tc.RecordingEncryptionServiceClient()
	if _, err := client.CompleteRotation(ctx, &recordingencryptionv1.CompleteRotationRequest{}); err != nil {
		return trace.Errorf("completing encryption key rotation: %v", err)
	}

	fmt.Fprintln(c.stdout, "Rotation completed")
	return nil
}

// Rollback an in progress key rotation.
func (c *recordingsEncryptionCommand) Rollback(ctx context.Context, tc *authclient.Client) error {
	client := tc.RecordingEncryptionServiceClient()
	if _, err := client.RollbackRotation(ctx, &recordingencryptionv1.RollbackRotationRequest{}); err != nil {
		return trace.Errorf("rolling back encryption key rotation: %v", err)
	}

	fmt.Fprintln(c.stdout, "Rotation rollback successful")
	return nil
}

// Status displays the current rotation status of the active encryption keys.
func (c *recordingsEncryptionCommand) Status(ctx context.Context, tc *authclient.Client) error {
	client := tc.RecordingEncryptionServiceClient()
	res, err := client.GetRotationState(ctx, &recordingencryptionv1.GetRotationStateRequest{})
	if err != nil {
		return trace.Errorf("fetching encryption key status: %v", err)
	}

	switch c.format {
	case teleport.Text, "":
		return trace.Wrap(c.writeStatusText(c.stdout, res.GetKeyPairStates()))
	case teleport.YAML:
		return trace.Wrap(c.writeStatusYAML(c.stdout, res.GetKeyPairStates()))
	case teleport.JSON:
		return trace.Wrap(c.writeStatusJSON(c.stdout, res.GetKeyPairStates()))
	}

	return trace.Wrap(err, "writing encryption key status")
}

func (c *recordingsEncryptionCommand) writeStatusJSON(w io.Writer, keyStates []*recordingencryptionv1.FingerprintWithState) error {
	data, err := json.MarshalIndent(keyStates, "", "    ")
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = w.Write(data)
	return trace.Wrap(err)
}

func (c *recordingsEncryptionCommand) writeStatusYAML(w io.Writer, keyStates []*recordingencryptionv1.FingerprintWithState) error {
	return trace.Wrap(utils.WriteYAML(w, keyStates))
}

func (c *recordingsEncryptionCommand) writeStatusText(w io.Writer, keyStates []*recordingencryptionv1.FingerprintWithState) error {
	rotationState := recordingencryptionv1.KeyPairState_KEY_PAIR_STATE_UNSPECIFIED
	t := asciitable.MakeTable([]string{"Key Pair Fingerprint", "State"})
	for _, pair := range keyStates {
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
		fmt.Fprintln(w, "Rotation failed due to inaccessible key")
	case recordingencryptionv1.KeyPairState_KEY_PAIR_STATE_ROTATING:
		fmt.Fprintln(w, "Rotation in progress")
	}

	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
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
