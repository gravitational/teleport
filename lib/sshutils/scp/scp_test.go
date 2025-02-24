/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package scp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

func TestSend(t *testing.T) {
	t.Parallel()
	modtime := testNow
	atime := testNow.Add(1 * time.Second)
	dirModtime := testNow.Add(2 * time.Second)
	dirAtime := testNow.Add(3 * time.Second)
	logger := utils.NewSlogLoggerForTests().With(teleport.ComponentKey, "send")
	testCases := []struct {
		desc   string
		config Config
		fs     *testFS
		args   []string
	}{
		{
			desc:   "regular file preserving the attributes",
			config: newSourceConfig("file", Flags{PreserveAttrs: true}),
			args:   args("-v", "-t", "-p"),
			fs:     newTestFS(logger, newFileTimes("file", modtime, atime, "file contents")),
		},
		{
			desc:   "directory preserving the attributes",
			config: newSourceConfig("dir", Flags{PreserveAttrs: true, Recursive: true}),
			args:   args("-v", "-t", "-r", "-p"),
			fs: newTestFS(
				logger,
				// Use timestamps extending backwards to test time application
				newDirTimes("dir", dirModtime.Add(1*time.Second), dirAtime.Add(2*time.Second),
					newFileTimes("dir/file", modtime.Add(1*time.Minute), atime.Add(2*time.Minute), "file contents"),
					newDirTimes("dir/dir2", dirModtime, dirAtime,
						newFileTimes("dir/dir2/file2", modtime, atime, "file2 contents")),
				),
			),
		},
	}
	for _, tt := range testCases {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()
			cmd, err := CreateCommand(tt.config)
			require.NoError(t, err)

			targetDir := t.TempDir()
			target := filepath.Join(targetDir, tt.config.Flags.Target[0])
			args := append(tt.args, target)

			// Source is missing, expect an error.
			err = runSCP(cmd, args...)
			require.Regexp(t, "could not access local path.*no such file or directory", err)

			tt.config.FileSystem = tt.fs
			cmd, err = CreateCommand(tt.config)
			require.NoError(t, err)
			// Resend the data
			err = runSCP(cmd, args...)
			require.NoError(t, err)

			fs := newEmptyTestFS(logger)
			fromOS(t, targetDir, fs)
			validateSCPTimes(t, fs, tt.fs)
			validateSCPContents(t, fs, tt.fs)
		})
	}
}

func TestReceive(t *testing.T) {
	t.Parallel()
	modtime := testNow
	atime := testNow.Add(1 * time.Second)
	dirModtime := testNow.Add(2 * time.Second)
	dirAtime := testNow.Add(3 * time.Second)
	logger := utils.NewSlogLoggerForTests().With(teleport.ComponentKey, "recv")
	testCases := []struct {
		desc       string
		config     Config
		source     string
		sourceFS   *testFS
		expectedFS *testFS
	}{
		{
			desc:     "regular file preserving the attributes",
			config:   newTargetConfig("file", Flags{PreserveAttrs: true}),
			source:   "file",
			sourceFS: newTestFS(logger, newFileTimes("file", modtime, atime, "file contents")),
		},
		{
			desc:   "directory preserving the attributes",
			config: newTargetConfig("dir", Flags{PreserveAttrs: true, Recursive: true}),
			source: "dir",
			sourceFS: newTestFS(
				logger,
				// Use timestamps extending backwards to test time application
				newDirTimes("dir", dirModtime.Add(1*time.Second), dirAtime.Add(2*time.Second),
					newFileTimes("dir/file", modtime.Add(1*time.Minute), atime.Add(2*time.Minute), "file contents"),
					newDirTimes("dir/dir2", dirModtime, dirAtime,
						newFileTimes("dir/dir2/file2", modtime, atime, "file2 contents")),
				),
			),
		},
		{
			desc:       "regular file into different filename (rename)",
			config:     newTargetConfig("remote_file", Flags{}),
			source:     "file",
			expectedFS: newTestFS(logger, newFile("remote_file", "file contents")),
			sourceFS:   newTestFS(logger, newFile("file", "file contents")),
		},
		{
			desc:       "regular file into different filename in a directory (rename)",
			config:     newTargetConfigWithFS("dir/remote_file", Flags{}, newTestFS(logger, newDir("dir"))),
			source:     "file",
			expectedFS: newTestFS(logger, newDir("dir", newFile("dir/remote_file", "file contents"))),
			sourceFS:   newTestFS(logger, newFile("file", "file contents")),
		},
		{
			desc:       "directory into different directory name (rename)",
			config:     newTargetConfig("remote_dir", Flags{Recursive: true}),
			source:     "dir",
			expectedFS: newTestFS(logger, newDir("remote_dir", newFile("remote_dir/file", "file contents"))),
			sourceFS:   newTestFS(logger, newDir("dir", newFile("dir/file", "file contents"))),
		},
		{
			desc:       "directory into different directory name in subdirectory (rename)",
			config:     newTargetConfigWithFS("dir/remote_dir", Flags{Recursive: true}, newTestFS(logger, newDir("dir"))),
			source:     "dir",
			expectedFS: newTestFS(logger, newDir("dir/remote_dir", newFile("dir/remote_dir/file", "file contents"))),
			sourceFS:   newTestFS(logger, newDir("dir", newFile("dir/file", "file contents"))),
		},
	}
	for _, tt := range testCases {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			logger := logger.With("test", tt.desc)
			t.Parallel()

			sourceDir := t.TempDir()
			source := filepath.Join(sourceDir, tt.source)
			args := []string{"-v", "-f"}
			if tt.config.Flags.PreserveAttrs {
				args = append(args, "-p")
			}
			if tt.config.Flags.Recursive {
				args = append(args, "-r")
			}
			args = append(args, source)

			if tt.config.FileSystem == nil {
				tt.config.FileSystem = newEmptyTestFS(logger)
			}
			cmd, err := CreateCommand(tt.config)
			require.NoError(t, err)

			writeData(t, sourceDir, tt.sourceFS)
			if tt.config.Flags.PreserveAttrs {
				writeFileTimes(t, sourceDir, tt.sourceFS)
			}

			// Send the data
			err = runSCP(cmd, args...)
			require.NoError(t, err)

			expectedFS := tt.sourceFS
			if tt.expectedFS != nil {
				expectedFS = tt.expectedFS
			}
			if tt.config.Flags.PreserveAttrs {
				validateSCPTimes(t, expectedFS, tt.config.FileSystem)
			} else {
				validateSCP(t, expectedFS, tt.config.FileSystem)
			}
			validateSCPContents(t, expectedFS, tt.config.FileSystem)
		})
	}
}

func TestSCPFailsIfNoSource(t *testing.T) {
	t.Parallel()
	config := newTargetConfig("file", Flags{})

	cmd, err := CreateCommand(config)
	require.NoError(t, err)

	sourceDir := t.TempDir()
	source := filepath.Join(sourceDir, config.Flags.Target[0])

	// Source is missing, expect an error.
	err = runSCP(cmd, "-v", "-f", source)
	require.Regexp(t, ".*No such file or directory", err)
}

// TestReceiveIntoExistingDirectory validates that the target remote directory
// is respected during copy.
//
// See https://github.com/gravitational/teleport/issues/5497
func TestReceiveIntoExistingDirectory(t *testing.T) {
	logger := utils.NewSlogLoggerForTests().With("test", t.Name())
	config := newTargetConfigWithFS("dir",
		Flags{PreserveAttrs: true, Recursive: true},
		newTestFS(logger, newDir("dir")),
	)
	sourceFS := newTestFS(
		logger,
		newDir("dir",
			newFile("dir/file", "file contents"),
			newDir("dir/dir2",
				newFile("dir/dir2/file2", "file2 contents")),
		),
	)
	expectedFS := newTestFS(
		logger,
		// Source is copied into an existing directory
		newDir("dir/dir",
			newFile("dir/dir/file", "file contents"),
			newDir("dir/dir/dir2",
				newFile("dir/dir/dir2/file2", "file2 contents")),
		),
	)
	sourceDir := t.TempDir()
	source := filepath.Join(sourceDir, config.Flags.Target[0])
	args := append(args("-v", "-f", "-r", "-p"), source)

	cmd, err := CreateCommand(config)
	require.NoError(t, err)

	writeData(t, sourceDir, sourceFS)

	err = runSCP(cmd, args...)
	require.NoError(t, err)

	validateSCP(t, expectedFS, config.FileSystem)
	validateSCPContents(t, expectedFS, config.FileSystem)
}

// TestReceiveIntoNonExistingDirectoryFailsWithCorrectMessage validates that copying a file into a non-existing
// directory fails with a correct error.
//
// See https://github.com/gravitational/teleport/issues/5695
func TestReceiveIntoNonExistingDirectoryFailsWithCorrectMessage(t *testing.T) {
	logger := utils.NewSlogLoggerForTests().With("test", t.Name())
	// Target configuration with no existing directory
	root := t.TempDir()
	config := newTargetConfigWithFS(filepath.Join(root, "dir"),
		Flags{PreserveAttrs: true},
		newTestFS(logger),
	)
	sourceFS := newTestFS(
		logger,
		newFile("file", "file contents"),
	)
	sourceDir := t.TempDir()
	source := filepath.Join(sourceDir, "file")
	args := append(args("-v", "-f"), source)

	cmd, err := CreateCommand(config)
	require.NoError(t, err)

	writeData(t, sourceDir, sourceFS)

	err = runSCP(cmd, args...)
	require.Error(t, err)
	require.Equal(t, fmt.Sprintf("no such file or directory %q", root), err.Error())
}

// TestCopyIntoNestedNonExistingDirectoriesDoesNotCreateIntermediateDirectories validates that copying a directory
// into a remote '/path/to/remote' where '/path/to' does not exist causes an error.
func TestCopyIntoNestedNonExistingDirectoriesDoesNotCreateIntermediateDirectories(t *testing.T) {
	logger := utils.NewSlogLoggerForTests().With("test", t.Name())

	config := newTargetConfig("non-existing/remote_dir", Flags{Recursive: true})
	sourceFS := newTestFS(logger, newDir("dir"))

	cmd, err := CreateCommand(config)
	require.NoError(t, err)

	sourceDir := t.TempDir()
	writeData(t, sourceDir, sourceFS)

	// Send the data
	err = runSCP(cmd, "-v", "-f", "-r", filepath.Join(sourceDir, "dir"))
	require.Error(t, err)
	require.Equal(t, "mkdir non-existing/remote_dir: no such file or directory", err.Error())
}

func TestInvalidDir(t *testing.T) {
	t.Parallel()

	cmd, err := CreateCommand(Config{
		User: "test-user",
		Flags: Flags{
			Sink: true,
			// Target is always defined
			Target:    []string{"./dir"},
			Recursive: true,
		},
	})
	require.NoError(t, err)

	testCases := []struct {
		desc      string
		inDirName string
		err       string
	}{
		{
			desc:      "no directory",
			inDirName: "",
			err:       ".*No such file or directory.*",
		},
		{
			desc:      "current directory",
			inDirName: ".",
			err:       ".*invalid name.*",
		},
		{
			desc:      "parent directory",
			inDirName: "..",
			err:       ".*invalid name.*",
		},
	}

	for _, tt := range testCases {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			scp, stdin, stdout, stderr := newCmd("scp", "-v", "-r", "-f", tt.inDirName)
			rw := &readWriter{r: stdout, w: stdin}

			doneC := make(chan struct{})
			// Service stderr
			go func() {
				io.Copy(io.Discard, stderr)
				close(doneC)
			}()

			err := scp.Start()
			require.NoError(t, err)

			err = cmd.Execute(rw)
			require.Regexp(t, tt.err, err)

			stdin.Close()
			<-doneC
			scp.Wait()
		})
	}
}

// TestVerifyDirectoryModeFailsWithFile makes sure that if scp was started in directory mode (the
// user attempts to copy multiple files or a directory), the target is a
// directory.
func TestVerifyDirectoryModeFailsWithFile(t *testing.T) {
	// Create temporary directory with a file "target" in it.
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	err := os.WriteFile(target, []byte{}, 0o666)
	require.NoError(t, err)

	cmd, err := CreateCommand(
		Config{
			User: "test-user",
			Flags: Flags{
				Source: true,
				Target: []string{target},
			},
		},
	)
	require.NoError(t, err)

	// Run command with -d flag (directory mode). Since the target is a file,
	// it should fail.
	err = runSCP(cmd, "-t", "-d", target)
	require.Regexp(t, ".*Not a directory", err)
}

// TestVerifyDirectoryModeIsRequiredForDirectory verifies that if a directory
// scp is attempted in non-recursive mode, the command fails as expected.
func TestVerifyDirectoryModeIsRequiredForDirectory(t *testing.T) {
	// Create temporary directory with a file "target" in it.
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	err := os.WriteFile(target, []byte{}, 0o666)
	require.NoError(t, err)

	cmd, err := CreateCommand(
		Config{
			User: "test-user",
			Flags: Flags{
				Source: true,
				Target: []string{dir},
			},
		},
	)
	require.NoError(t, err)

	// Run command in non-recursive mode. Since the source is a directory,
	// it should fail.
	err = runSCP(cmd, "-t", dir)
	require.Regexp(t, fmt.Sprintf("%s is a directory, use -r flag to copy recursively", filepath.Base(dir)), err)
}

func runSCP(cmd Command, args ...string) error {
	scp, stdin, stdout, _ := newCmd("scp", args...)
	rw := &readWriter{r: stdout, w: stdin}

	errCh := make(chan error, 1)

	go func() {
		defer close(errCh)
		if err := scp.Start(); err != nil {
			errCh <- err
			return
		}
		if err := cmd.Execute(rw); err != nil {
			errCh <- err
			return
		}
		stdin.Close()
		if err := scp.Wait(); err != nil {
			errCh <- err
			return
		}
	}()

	select {
	case <-time.After(2 * time.Second):
		return trace.BadParameter("timed out waiting for command")
	case err := <-errCh:
		if err == nil {
			return nil
		}
		return trace.Wrap(err)
	}
}

// fromOS recreates the structure of the specified directory dir
// into the provided file system fs
func fromOS(t *testing.T, dir string, fs *testFS) {
	err := filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relpath, err := filepath.Rel(dir, path)
		require.NoError(t, err)
		if relpath == "." {
			// Skip top-level directory
			return nil
		}
		if fi.IsDir() {
			require.NoError(t, fs.MkDir(relpath, int(fi.Mode())))
			require.NoError(t, fs.Chtimes(relpath, GetAtime(fi), fi.ModTime()))
			return nil
		}
		wc, err := fs.CreateFile(relpath, uint64(fi.Size()))
		require.NoError(t, err)
		defer wc.Close()
		require.NoError(t, fs.Chtimes(relpath, GetAtime(fi), fi.ModTime()))
		f, err := os.Open(path)
		require.NoError(t, err)
		defer f.Close()
		_, err = io.Copy(wc, f)
		require.NoError(t, err)
		return nil
	})
	require.NoError(t, err)
}

// writeData recreates the file/directory structure in dir
// as specified with the file system fs
func writeData(t *testing.T, dir string, fs *testFS) {
	for _, f := range fs.fs {
		if f.IsDir() {
			require.NoError(t, os.MkdirAll(filepath.Join(dir, f.path), f.perms))
			continue
		}
		rc, err := fs.OpenFile(f.path)
		require.NoError(t, err)
		defer rc.Close()
		targetPath := filepath.Join(dir, f.path)
		if parentDir := filepath.Dir(f.path); parentDir != "." {
			fi := fs.fs[parentDir]
			require.NoError(t, os.MkdirAll(filepath.Dir(targetPath), fi.perms))
		}
		f, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY, f.perms)
		require.NoError(t, err)
		defer f.Close()
		_, err = io.Copy(f, rc)
		require.NoError(t, err)
	}
}

// writeFileTimes applies access/modification times on files/directories in dir
// as specified in the file system fs.
func writeFileTimes(t *testing.T, dir string, fs *testFS) {
	for _, f := range fs.fs {
		require.NoError(t, os.Chtimes(filepath.Join(dir, f.path), f.atime, f.modtime))
	}
}

// validateSCPContents verifies that the file contents in the specified
// file systems match in the corresponding files
func validateSCPContents(t *testing.T, expected *testFS, actual FileSystem) {
	for path, fileinfo := range expected.fs {
		if fileinfo.IsDir() {
			continue
		}
		rc, err := actual.OpenFile(path)
		require.NoError(t, err)
		defer rc.Close()
		bytes, err := io.ReadAll(rc)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(fileinfo.contents.String(), string(bytes)))
	}
}

// validateSCP verifies that the specified pair of FileSystems match.
func validateSCP(t *testing.T, expected *testFS, actual FileSystem) {
	for path, fileinfo := range expected.fs {
		targetFileinfo, err := actual.GetFileInfo(path)
		require.NoError(t, err, "expected %v (%v)", path, fileinfo)
		if fileinfo.IsDir() {
			require.True(t, targetFileinfo.IsDir())
		} else {
			require.True(t, targetFileinfo.GetModePerm().IsRegular())
		}
	}
}

// validateSCPTimes verifies that the specified pair of FileSystems match.
// FileSystem match if their contents match incl. access/modification times
func validateSCPTimes(t *testing.T, expected *testFS, actual FileSystem) {
	for path, fileinfo := range expected.fs {
		targetFileinfo, err := actual.GetFileInfo(path)
		require.NoError(t, err, "expected %v (%v)", path, fileinfo)
		if fileinfo.IsDir() {
			require.True(t, targetFileinfo.IsDir())
		} else {
			require.True(t, targetFileinfo.GetModePerm().IsRegular())
		}
		validateFileTimes(t, fileinfo, targetFileinfo)
	}
}

// validateFileTimes verifies that the specified pair of FileInfos match
func validateFileTimes(t *testing.T, expected *testFileInfo, actual FileInfo) {
	require.Empty(t, cmp.Diff(
		expected.GetModTime().UTC().Format(time.RFC3339),
		actual.GetModTime().UTC().Format(time.RFC3339),
	), "validating modification times for %v", actual)
	require.Empty(t, cmp.Diff(
		expected.GetAccessTime().UTC().Format(time.RFC3339),
		actual.GetAccessTime().UTC().Format(time.RFC3339),
	), "validating access times for %v", actual)
}

type readWriter struct {
	r io.Reader
	w io.Writer
}

func (c *readWriter) Read(b []byte) (int, error) {
	return c.r.Read(b)
}

func (c *readWriter) Write(b []byte) (int, error) {
	return c.w.Write(b)
}

func newCmd(name string, args ...string) (cmd *exec.Cmd, stdin io.WriteCloser, stdout io.ReadCloser, stderr io.ReadCloser) {
	cmd = exec.Command(name, args...)

	var err error
	stdin, err = cmd.StdinPipe()
	if err != nil {
		panic(err)
	}

	stdout, err = cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}

	stderr, err = cmd.StderrPipe()
	if err != nil {
		panic(err)
	}

	return cmd, stdin, stdout, stderr
}

// newTestFS creates a new test FileSystem using the specified logger
// and the set of top-level files
func newTestFS(logger *slog.Logger, files ...*testFileInfo) *testFS {
	fs := newEmptyTestFS(logger)
	addFiles(fs.fs, files...)
	return fs
}

// newEmptyTestFS creates a new test FileSystem without content
func newEmptyTestFS(logger *slog.Logger) *testFS {
	return &testFS{
		fs: make(map[string]*testFileInfo),
		l:  logger,
	}
}

func (r *testFS) IsDir(path string) bool {
	r.l.DebugContext(context.Background(), "IsDir", "path", path)
	if fi, exists := r.fs[path]; exists {
		return fi.IsDir()
	}
	return false
}

func (r *testFS) GetFileInfo(path string) (FileInfo, error) {
	r.l.DebugContext(context.Background(), "GetFileInfo", "path", path)
	fi, exists := r.fs[path]
	if !exists {
		return nil, newErrMissingFile(path)
	}
	return fi, nil
}

func (r *testFS) MkDir(path string, mode int) error {
	r.l.DebugContext(context.Background(), "MkDir", "path", path, "mode", mode)
	_, exists := r.fs[path]
	if exists {
		return trace.AlreadyExists("directory %v already exists", path)
	}
	r.fs[path] = &testFileInfo{
		path:  path,
		dir:   true,
		perms: os.FileMode(mode) | os.ModeDir,
	}
	return nil
}

func (r *testFS) OpenFile(path string) (io.ReadCloser, error) {
	r.l.DebugContext(context.Background(), "OpenFile", "path", path)
	fi, exists := r.fs[path]
	if !exists {
		return nil, newErrMissingFile(path)
	}
	rc := nopReadCloser{Reader: bytes.NewReader(fi.contents.Bytes())}
	return rc, nil
}

func (r *testFS) CreateFile(path string, length uint64) (io.WriteCloser, error) {
	r.l.DebugContext(context.Background(), "CreateFile", "path", path, "len", length)
	baseDir := filepath.Dir(path)
	if _, exists := r.fs[baseDir]; baseDir != "." && !exists {
		return nil, newErrMissingFile(baseDir)
	}
	fi := &testFileInfo{
		path:     path,
		size:     int64(length),
		perms:    0o666,
		contents: new(bytes.Buffer),
	}
	r.fs[path] = fi
	wc := utils.NopWriteCloser(fi.contents)
	return wc, nil
}

func (r *testFS) Chmod(path string, mode int) error {
	r.l.DebugContext(context.Background(), "Chmod", "path", path, "mode", mode)
	fi, exists := r.fs[path]
	if !exists {
		return newErrMissingFile(path)
	}
	fi.perms = os.FileMode(mode)
	return nil
}

func (r *testFS) Chtimes(path string, atime, mtime time.Time) error {
	r.l.DebugContext(context.Background(), "Chtimes", "path", path, "atime", atime, "mtime", mtime)
	fi, exists := r.fs[path]
	if !exists {
		return newErrMissingFile(path)
	}
	fi.modtime = mtime
	fi.atime = atime
	return nil
}

// testFS implements a fake FileSystem
type testFS struct {
	l  *slog.Logger
	fs map[string]*testFileInfo
}

type testFileInfo struct {
	dir      bool
	perms    os.FileMode
	path     string
	modtime  time.Time
	atime    time.Time
	ents     []*testFileInfo
	size     int64
	contents *bytes.Buffer
}

func (r *testFileInfo) String() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "fileinfo(path=%s,perms=%d,size=%d", r.path, r.perms, r.size)
	if r.dir {
		fmt.Fprintf(&buf, ",dir(ents=%d)", len(r.ents))
	}
	fmt.Fprint(&buf, ")")
	return buf.String()
}
func (r *testFileInfo) IsDir() bool { return r.dir }
func (r *testFileInfo) ReadDir() (fis []FileInfo, err error) {
	fis = make([]FileInfo, 0, len(r.ents))
	for _, e := range r.ents {
		fis = append(fis, e)
	}
	return fis, nil
}
func (r *testFileInfo) GetName() string          { return filepath.Base(r.path) }
func (r *testFileInfo) GetPath() string          { return r.path }
func (r *testFileInfo) GetModePerm() os.FileMode { return r.perms }
func (r *testFileInfo) GetSize() int64           { return r.size }
func (r *testFileInfo) GetModTime() time.Time    { return r.modtime }
func (r *testFileInfo) GetAccessTime() time.Time { return r.atime }

func (r nopReadCloser) Close() error { return nil }

type nopReadCloser struct {
	io.Reader
}

func newErrMissingFile(path string) error {
	return fmt.Errorf("no such file or directory %q", path)
}

func newSourceConfig(path string, flags Flags) Config {
	flags.Source = true
	flags.Target = []string{path}
	return Config{
		User:  "test-user",
		Flags: flags,
	}
}

func newTargetConfigWithFS(path string, flags Flags, fs *testFS) Config {
	config := newTargetConfig(path, flags)
	config.FileSystem = fs
	return config
}

func newTargetConfig(path string, flags Flags) Config {
	flags.Sink = true
	flags.Target = []string{path}
	return Config{
		User:  "test-user",
		Flags: flags,
	}
}

func newDir(name string, ents ...*testFileInfo) *testFileInfo {
	return &testFileInfo{
		path:  name,
		ents:  ents,
		dir:   true,
		perms: 0o755,
	}
}

func newFile(name string, contents string) *testFileInfo {
	return &testFileInfo{
		path:     name,
		perms:    0o666,
		size:     int64(len(contents)),
		contents: bytes.NewBufferString(contents),
	}
}

func newDirTimes(name string, modtime, atime time.Time, ents ...*testFileInfo) *testFileInfo {
	return &testFileInfo{
		path:    name,
		ents:    ents,
		modtime: modtime,
		atime:   atime,
		dir:     true,
		perms:   0o755,
	}
}

func newFileTimes(name string, modtime, atime time.Time, contents string) *testFileInfo {
	return &testFileInfo{
		path:     name,
		modtime:  modtime,
		atime:    atime,
		perms:    0o666,
		size:     int64(len(contents)),
		contents: bytes.NewBufferString(contents),
	}
}

func addFiles(fs map[string]*testFileInfo, ents ...*testFileInfo) {
	for _, f := range ents {
		fs[f.path] = f
		if f.IsDir() {
			addFiles(fs, f.ents...)
		}
	}
}

var testNow = time.Date(1984, time.April, 4, 0, 0, 0, 0, time.UTC)

func args(params ...string) []string {
	return params
}
