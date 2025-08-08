package common

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	recordingencryptionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestRecordingEncryptionKeyRotation(t *testing.T) {
	dynAddr := helpers.NewDynamicServiceAddr(t)
	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.AuthAddr,
			},
			SessionRecordingConfig: &types.SessionRecordingConfigSpecV2{
				Mode: "node",
				Encryption: &types.SessionRecordingEncryptionConfig{
					Enabled: true,
				},
			},
		},
	}

	process := makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.Descriptors))
	clt := testenv.MakeDefaultAuthClient(t, process)

	// get initial status to confirm one active key exists
	keyStates := getEncryptionKeyStates(t, clt)
	require.Len(t, keyStates, 1)
	initialKeyState := keyStates[0]
	require.Equal(t, recordingencryptionv1.KeyPairState_KEY_PAIR_STATE_ACTIVE, initialKeyState.State)

	// start key rotation
	_, err := runRecordingsCommand(t, clt, []string{"recordings", "encryption", "rotate"})
	require.NoError(t, err)

	// refetch status to confirm original key is now 'rotating' and new key is 'active'
	keyStates = getEncryptionKeyStates(t, clt)
	require.Len(t, keyStates, 2)
	rotatedKeyState := keyStates[0]
	newKeyState := keyStates[1]

	require.Equal(t, initialKeyState.Fingerprint, rotatedKeyState.Fingerprint)
	require.Equal(t, recordingencryptionv1.KeyPairState_KEY_PAIR_STATE_ROTATING, rotatedKeyState.State)
	require.Equal(t, recordingencryptionv1.KeyPairState_KEY_PAIR_STATE_ACTIVE, newKeyState.State)

	// confirm a second rotation fails when one is already in progress
	_, err = runRecordingsCommand(t, clt, []string{"recordings", "encryption", "rotate"})
	require.Error(t, err)

	// rollback rotation
	_, err = runRecordingsCommand(t, clt, []string{"recordings", "encryption", "rollback"})
	require.NoError(t, err)

	// ensure initial key is the only active key remaining
	keyStates = getEncryptionKeyStates(t, clt)
	require.Len(t, keyStates, 1)
	newKeyState = keyStates[0]
	require.Equal(t, initialKeyState.Fingerprint, newKeyState.Fingerprint)
	require.Equal(t, recordingencryptionv1.KeyPairState_KEY_PAIR_STATE_ACTIVE, newKeyState.State)

	// start a new rotation
	_, err = runRecordingsCommand(t, clt, []string{"recordings", "encryption", "rotate"})
	require.NoError(t, err)

	// confirm in progress rotation state
	keyStates = getEncryptionKeyStates(t, clt)
	require.Len(t, keyStates, 2)
	rotatedKeyState = keyStates[0]
	newKeyState = keyStates[1]
	require.Equal(t, initialKeyState.Fingerprint, rotatedKeyState.Fingerprint)
	require.Equal(t, recordingencryptionv1.KeyPairState_KEY_PAIR_STATE_ROTATING, rotatedKeyState.State)
	require.Equal(t, recordingencryptionv1.KeyPairState_KEY_PAIR_STATE_ACTIVE, newKeyState.State)

	// complete rotation
	_, err = runRecordingsCommand(t, clt, []string{"recordings", "encryption", "complete"})
	require.NoError(t, err)

	// ensure remaining active key is new
	keyStates = getEncryptionKeyStates(t, clt)
	require.Len(t, keyStates, 1)
	finalKeyState := keyStates[0]
	require.Equal(t, newKeyState.Fingerprint, finalKeyState.Fingerprint)
	require.Equal(t, recordingencryptionv1.KeyPairState_KEY_PAIR_STATE_ACTIVE, finalKeyState.State)
}

func getEncryptionKeyStates(t *testing.T, client *authclient.Client) []*recordingencryptionv1.FingerprintWithState {
	var keyStates []*recordingencryptionv1.FingerprintWithState
	out, err := runRecordingsCommand(t, client, []string{"recordings", "encryption", "status", "--format", "json"})
	require.NoError(t, err)
	err = json.Unmarshal(out.Bytes(), &keyStates)
	require.NoError(t, err)

	return keyStates
}

func runRecordingsCommand(t *testing.T, client *authclient.Client, args []string) (*bytes.Buffer, error) {
	var stdoutBuff bytes.Buffer
	command := &RecordingsCommand{
		stdout: &stdoutBuff,
	}

	return &stdoutBuff, runCommand(t, client, command, args)
}
