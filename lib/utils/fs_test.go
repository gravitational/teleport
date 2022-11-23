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

package utils

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestLocks(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "teleport-lock-test")
	fp := tmpFile.Name()
	t.Cleanup(func() {
		_ = os.Remove(fp)
	})
	require.NoError(t, err)

	// Can take read lock
	unlock, err := FSTryReadLock(fp)
	require.NoError(t, err)

	require.NoError(t, unlock())

	// Can take write lock
	unlock, err = FSTryWriteLock(fp)
	require.NoError(t, err)

	// Can't take read lock while write lock is held.
	unlock2, err := FSTryReadLock(fp)
	require.ErrorIs(t, err, ErrUnsuccessfulLockTry)
	require.Nil(t, unlock2)

	// Can't take write lock while another write lock is held.
	unlock2, err = FSTryWriteLock(fp)
	require.ErrorIs(t, err, ErrUnsuccessfulLockTry)
	require.Nil(t, unlock2)

	require.NoError(t, unlock())

	unlock, err = FSTryReadLock(fp)
	require.NoError(t, err)

	// Can take second read lock on the same file.
	unlock2, err = FSTryReadLock(fp)
	require.NoError(t, err)

	require.NoError(t, unlock())
	require.NoError(t, unlock2())

	// Can take read lock with timeout
	unlock, err = FSTryReadLockTimeout(context.Background(), fp, time.Second)
	require.NoError(t, err)
	require.NoError(t, unlock())

	// Can take write lock with timeout
	unlock, err = FSTryWriteLockTimeout(context.Background(), fp, time.Second)
	require.NoError(t, err)

	// Fails because timeout is exceeded, since file is already locked.
	unlock2, err = FSTryWriteLockTimeout(context.Background(), fp, time.Millisecond)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Nil(t, unlock2)

	// Fails because context is expired while waiting for timeout.
	ctx, cancel := context.WithDeadline(context.Background(), time.Now())
	defer cancel()
	unlock2, err = FSTryWriteLockTimeout(ctx, fp, time.Hour*1000)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Nil(t, unlock2)

	require.NoError(t, unlock())
}

func TestOverwriteFile(t *testing.T) {
	have := []byte("Sensitive Information")

	f, err := os.Create(filepath.Join(t.TempDir(), "teleport-overwrite-file-test"))
	if err != nil {
		t.Fatalf("Unable to create tmp file: %s\n", err)
	}

	if _, err := f.Write(have); err != nil {
		f.Close()
		t.Fatalf("Error writing to tmp file: %s\n", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Unable to close tmp file: %s\n", err)
	}

	if err := overwriteFile(f.Name()); err != nil {
		t.Fatalf("Unable to overwrite tmp file: %s\n", err)
	}

	contents, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatalf("Unable to read tmp file: %s\n", err)
	}

	if bytes.Contains(contents, have) {
		t.Fatal("File contents were not overwritten")
	}
}

func TestRemoveSecure(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "teleport-remove-secure-test"))
	if err != nil {
		t.Fatalf("Unable to create tmp file: %s\n", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Unable to close tmp file: %s\n", err)
	}
	if err := RemoveSecure(f.Name(), 0); !trace.IsBadParameter(err) {
		t.Fatalf("RemoveSecure(filePath, 0) = %v; expected trace.BadParameterError\n", err)
	}
	if err := RemoveSecure(f.Name(), 1); err != nil {
		t.Fatalf("RemoveSecure(filePath, 1) = %v; expected nil\n", err)
	}
}
