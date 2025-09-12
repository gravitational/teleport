/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package agent

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"unicode"

	"github.com/gravitational/trace"
)

const (
	// fileHeaderSniffBytes is the max size to read to determine a file's MIME type
	fileHeaderSniffBytes = 512 // MIME standard size
	// execModeMask is the minimum required set of bits to consider a file executable.
	execModeMask = 0111
)

// Validator validates filesystem paths.
type Validator struct {
	Log *slog.Logger
}

// IsBinary returns true for working binaries that are executable by all users.
// If the file is irregular, non-executable, or a shell script, IsBinary returns false and logs a warning.
// IsBinary errors if lstat fails, a regular file is unreadable, or an executable file does not execute.
func (v *Validator) IsBinary(ctx context.Context, path string) (bool, error) {
	// The behavior of this method is intended to protect against executable files
	// being adding to the Teleport tgz that should not be made available on PATH,
	// and additionally, should not cause installation to fail.
	// While known copies of these files (e.g., "install") are excluded during extraction,
	// it is safer to assume others could be present in past or future tgzs.

	if exec, err := v.IsExecutable(ctx, path); err != nil || !exec {
		return exec, trace.Wrap(err)
	}
	name := filepath.Base(path)
	d, err := readFileLimit(path, fileHeaderSniffBytes)
	if err != nil {
		return false, trace.Wrap(err)
	}
	// Refuse to test or link shell scripts
	if isTextScript(d) {
		v.Log.WarnContext(ctx, "Found unexpected shell script", "name", name)
		return false, nil
	}
	v.Log.InfoContext(ctx, "Validating binary", "name", name)
	r := localExec{
		Log:      v.Log,
		ErrLevel: slog.LevelDebug,
		OutLevel: slog.LevelInfo, // always show version
	}
	code, err := r.Run(ctx, path, "version")
	if code < 0 {
		return false, trace.Wrap(err, "error validating binary %s", name)
	}
	if code > 0 {
		v.Log.InfoContext(ctx, "Binary does not support version command", "name", name)
	}
	return true, nil
}

// IsExecutable returns true for regular, executable files.
func (v *Validator) IsExecutable(ctx context.Context, path string) (bool, error) {
	name := filepath.Base(path)
	fi, err := os.Lstat(path)
	if err != nil {
		return false, trace.Wrap(err)
	}
	if !fi.Mode().IsRegular() {
		v.Log.WarnContext(ctx, "Found unexpected irregular file", "name", name)
		return false, nil
	}
	if fi.Mode()&execModeMask != execModeMask {
		v.Log.WarnContext(ctx, "Found unexpected non-executable file", "name", name)
		return false, nil
	}
	return true, nil
}

func isTextScript(data []byte) bool {
	data = bytes.TrimLeftFunc(data, unicode.IsSpace)
	if !bytes.HasPrefix(data, []byte("#!")) {
		return false
	}
	// Assume presence of MIME binary data bytes means binary:
	//   https://mimesniff.spec.whatwg.org/#terminology
	for _, b := range data {
		switch {
		case b <= 0x08, b == 0x0B,
			0x0E <= b && b <= 0x1A,
			0x1C <= b && b <= 0x1F:
			return false
		}
	}
	return true
}

// readFileLimit the first n bytes of a file, or less if shorter.
func readFileLimit(name string, n int64) ([]byte, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var buf bytes.Buffer
	_, err = io.Copy(&buf, io.LimitReader(f, n))
	return buf.Bytes(), trace.Wrap(err)
}
