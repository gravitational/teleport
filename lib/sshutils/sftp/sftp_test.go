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

package sftp

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"fmt"
	"io"
	mathrand "math/rand/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

const fileMaxSize = 1000

func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)
	os.Exit(m.Run())
}

func TestTransferFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		req             *FileTransferRequest
		globbedSrcPaths []string
		files           []string
		errCheck        require.ErrorAssertionFunc
	}{
		{
			name: "one file",
			req: &FileTransferRequest{
				Sources: Sources{
					Paths: []string{"file"},
				},
				Destination: Target{
					Path: "copied-file",
				},
				PreserveAttrs: true,
			},
			files: []string{
				"file",
			},
		},
		{
			name: "one file to dir",
			req: &FileTransferRequest{
				Sources: Sources{
					Paths: []string{"file"},
				},
				Destination: Target{
					Path: "dst/",
				},
				PreserveAttrs: true,
			},
			files: []string{
				"file",
				"dst/",
			},
		},
		{
			name: "one dir",
			req: &FileTransferRequest{
				Sources: Sources{
					Paths: []string{"src/"},
				},
				Destination: Target{
					Path: "dir/",
				},
				PreserveAttrs: true,
				Recursive:     true,
			},
			files: []string{
				"src/",
			},
		},
		{
			name: "two files dst doesn't exist",
			req: &FileTransferRequest{
				Sources: Sources{
					Paths: []string{
						"src/file1",
						"src/file2",
					},
				},
				Destination: Target{
					Path: "dst/",
				},
				PreserveAttrs: true,
			},
			files: []string{
				"src/file1",
				"src/file2",
			},
		},
		{
			name: "two files dst does exist",
			req: &FileTransferRequest{
				Sources: Sources{
					Paths: []string{
						"src/file1",
						"src/file2",
					},
				},
				Destination: Target{
					Path: "dst/",
				},
				PreserveAttrs: true,
			},
			files: []string{
				"src/file1",
				"src/file2",
				"dst/",
			},
		},
		{
			name: "nested dirs",
			req: &FileTransferRequest{
				Sources: Sources{
					Paths: []string{"s"},
				},
				Destination: Target{
					Path: "dst/",
				},
				PreserveAttrs: true,
				Recursive:     true,
			},
			files: []string{
				"s/",
				"s/file",
				"s/r/",
				"s/r/file",
				"s/r/c/",
				"s/r/c/file",
				"dst/",
			},
		},
		{
			name: "globbed files dst doesn't exist",
			req: &FileTransferRequest{
				Sources: Sources{
					Paths: []string{"glob*"},
				},
				Destination: Target{
					Path: "dst/",
				},
			},
			globbedSrcPaths: []string{
				"globS",
				"globA",
				"globT",
				"globB",
			},
			files: []string{
				"globS",
				"globA",
				"globT",
				"globB",
			},
		},
		{
			name: "globbed files dst does exist",
			req: &FileTransferRequest{
				Sources: Sources{
					Paths: []string{"glob*"},
				},
				Destination: Target{
					Path: "dst/",
				},
			},
			globbedSrcPaths: []string{
				"globS",
				"globA",
				"globT",
				"globB",
			},
			files: []string{
				"globS",
				"globA",
				"globT",
				"globB",
				"dst/",
			},
		},
		{
			name: "multiple glob patterns",
			req: &FileTransferRequest{
				Sources: Sources{
					Paths: []string{
						"glob*",
						"*stuff",
					},
				},
				Destination: Target{
					Path: "dst/",
				},
			},
			globbedSrcPaths: []string{
				"globS",
				"globA",
				"globT",
				"globB",
				"mystuff",
				"yourstuff",
			},
			files: []string{
				"globS",
				"globA",
				"globT",
				"globB",
				"mystuff",
				"yourstuff",
				"dst/",
			},
		},
		{
			name: "multiple glob patterns with normal path",
			req: &FileTransferRequest{
				Sources: Sources{
					Paths: []string{
						"glob*",
						"file",
						"*stuff",
					},
				},
				Destination: Target{
					Path: "dst/",
				},
			},
			globbedSrcPaths: []string{
				"globS",
				"globA",
				"globT",
				"globB",
				"file",
				"mystuff",
				"yourstuff",
			},
			files: []string{
				"globS",
				"globA",
				"globT",
				"globB",
				"file",
				"mystuff",
				"yourstuff",
				"dst/",
			},
		},
		{
			name: "recursive glob pattern",
			req: &FileTransferRequest{
				Sources: Sources{
					Paths: []string{"glob*"},
				},
				Destination: Target{
					Path: "dst/",
				},
				Recursive:     true,
				PreserveAttrs: true,
			},
			globbedSrcPaths: []string{
				"globS",
				"globA",
				"globT",
				"globB",
				"globfile",
			},
			files: []string{
				"globS/",
				"globS/file",
				"globA/",
				"globA/file",
				"globT/",
				"globT/file",
				"globB/",
				"globB/file",
				"globfile",
				"dst/",
			},
		},
		{
			name: "recursive glob pattern with normal path",
			req: &FileTransferRequest{
				Sources: Sources{
					Paths: []string{
						"glob*",
						"file",
					},
				},
				Destination: Target{
					Path: "dst/",
				},
				PreserveAttrs: true,
				Recursive:     true,
			},
			globbedSrcPaths: []string{
				"globS",
				"globA",
				"globT",
				"globB",
				"globfile",
				"file",
			},
			files: []string{
				"globS/",
				"globS/file",
				"globA/",
				"globA/file",
				"globT/",
				"globT/file",
				"globB/",
				"globB/file",
				"globfile",
				"file",
				"dst/",
			},
		},
		{
			name: "multiple src dst not dir",
			req: &FileTransferRequest{
				Sources: Sources{
					Paths: []string{
						"uno",
						"dos",
						"tres",
					},
				},
				Destination: Target{
					Path: "dst_file",
				},
			},
			files: []string{
				"uno",
				"dos",
				"tres",
				"dst_file",
			},
			errCheck: func(t require.TestingT, err error, i ...any) {
				require.EqualError(t, err, fmt.Sprintf(`local file "%s/dst_file" is not a directory, but multiple source files were specified`, i[0]))
			},
		},
		{
			name: "multiple matches from src dst not dir",
			req: &FileTransferRequest{
				Sources: Sources{
					Paths: []string{"glob*"},
				},
				Destination: Target{
					Path: "dst_file",
				},
			},
			files: []string{
				"glob1",
				"glob2",
				"glob3",
				"dst_file",
			},
			errCheck: func(t require.TestingT, err error, i ...any) {
				require.EqualError(t, err, fmt.Sprintf(`local file "%s/dst_file" is not a directory, but multiple source files were matched by a glob pattern`, i[0]))
			},
		},
		{
			name: "src dir with recursive not passed",
			req: &FileTransferRequest{
				Sources: Sources{
					Paths: []string{"src/"},
				},
				Destination: Target{
					Path: "dst/",
				},
			},
			files: []string{
				"src/",
			},
			errCheck: func(t require.TestingT, err error, i ...any) {
				require.EqualError(t, err, fmt.Sprintf(`"%s/src" is a directory, but the recursive option was not passed`, i[0]))
				require.ErrorAs(t, err, new(*NonRecursiveDirectoryTransferError))
			},
		},
		{
			name: "non-existent src file",
			req: &FileTransferRequest{
				Sources: Sources{
					Paths: []string{"idontexist"},
				},
				Destination: Target{
					Path: "whocares",
				},
			},
			errCheck: func(t require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, os.ErrNotExist)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// create necessary files
			tempDir := t.TempDir()
			for _, file := range tt.files {
				// if path ends in slash, create dir
				if strings.HasSuffix(file, string(filepath.Separator)) {
					createDir(t, filepath.Join(tempDir, file))
				} else {
					createFile(t, filepath.Join(tempDir, file))
				}
			}
			for i, src := range tt.req.Sources.Paths {
				tt.req.Sources.Paths[i] = filepath.Join(tempDir, src)
			}
			for i := range tt.globbedSrcPaths {
				tt.globbedSrcPaths[i] = filepath.Join(tempDir, tt.globbedSrcPaths[i])
			}
			tt.req.Destination.Path = filepath.Join(tempDir, tt.req.Destination.Path)

			ctx := context.Background()
			err := TransferFiles(ctx, tt.req)
			if tt.errCheck == nil {
				require.NoError(t, err)
				srcPaths := tt.req.Sources.Paths
				if len(tt.globbedSrcPaths) != 0 {
					srcPaths = tt.globbedSrcPaths
				}
				checkTransfer(t, tt.req.PreserveAttrs, tt.req.Destination.Path, srcPaths...)
			} else {
				tt.errCheck(t, err, tempDir)
			}
		})
	}
}

func TestCopyingSymlinkedFile(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	createFile(t, filepath.Join(tempDir, "file"))
	linkPath := filepath.Join(tempDir, "link")
	err := os.Symlink(filepath.Join(tempDir, "file"), linkPath)
	require.NoError(t, err)

	dstPath := filepath.Join(tempDir, "dst")
	req := &FileTransferRequest{
		Sources: Sources{
			Paths: []string{linkPath},
		},
		Destination: Target{
			Path: dstPath,
		},
	}

	err = TransferFiles(t.Context(), req)
	require.NoError(t, err)

	checkTransfer(t, false, dstPath, linkPath)
}

type mockFile struct {
	File
	altDataSource io.Reader
}

func (m *mockFile) Read(p []byte) (int, error) {
	return m.altDataSource.Read(p)
}

type mockFS struct {
	localFS
	fileAccesses map[string]int
	altData      io.Reader
}

func (m *mockFS) Open(path string) (File, error) {
	if m.fileAccesses == nil {
		m.fileAccesses = make(map[string]int)
	}
	realPath, err := m.localFS.RealPath(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	m.fileAccesses[realPath]++
	file, err := m.localFS.Open(path)
	if err != nil || m.altData == nil {
		return file, err
	}
	return &mockFile{
		File:          file,
		altDataSource: m.altData,
	}, nil
}

func TestRecursiveSymlinks(t *testing.T) {
	// Create files and symlinks.
	root := t.TempDir()
	t.Chdir(root)
	srcDir := filepath.Join(root, "a")
	createDir(t, filepath.Join(srcDir, "b/c"))
	fileA := "a/a.txt"
	fileB := "a/b/b.txt"
	fileC := "a/b/c/c.txt"
	for _, file := range []string{fileA, fileB, fileC} {
		createFile(t, filepath.Join(root, file))
	}
	require.NoError(t, os.Symlink(srcDir, filepath.Join(srcDir, "abs_link")))
	require.NoError(t, os.Symlink("..", filepath.Join(srcDir, "b/rel_link")))

	tests := []struct {
		name   string
		srcDir string
	}{
		{
			name:   "absolute",
			srcDir: srcDir,
		},
		{
			name:   "relative",
			srcDir: filepath.Base(srcDir),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Perform the transfer.
			dstDir := filepath.Join(root, "dst")
			t.Cleanup(func() { os.RemoveAll(dstDir) })

			srcFS := &mockFS{}
			req := &FileTransferRequest{
				Sources: Sources{
					Paths: []string{tc.srcDir},
				},
				Destination: Target{
					Path: dstDir,
				},
				Recursive: true,
				srcFS:     srcFS,
			}
			require.NoError(t, TransferFiles(t.Context(), req))

			// Check results. Don't use checkTransfer() as the directories will not have
			// matching sizes (the symlinks that aren't copied over).
			for _, file := range []string{fileA, fileB, fileC} {
				srcFile, err := srcFS.RealPath(filepath.Join(filepath.Dir(tc.srcDir), file))
				require.NoError(t, err)
				srcInfo, err := os.Stat(srcFile)
				require.NoError(t, err)
				dstFile, err := srcFS.RealPath(filepath.Join(dstDir, file))
				require.NoError(t, err)
				dstInfo, err := os.Stat(dstFile)
				require.NoError(t, err)
				compareFiles(t, false, dstInfo, srcInfo, dstFile, srcFile)
				// Check that the file was only opened once.
				accesses := srcFS.fileAccesses[srcFile]
				require.Equal(t, 1, accesses, "file %q was opened %d times", srcFile, accesses)
			}
		})
	}
}

func TestHTTPUpload(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	src := filepath.Join(tempDir, "source")
	dst := filepath.Join(tempDir, "destination")

	createFile(t, src)
	f, err := os.Open(src)
	require.NoError(t, err)
	t.Cleanup(func() {
		f.Close()
	})

	req, err := http.NewRequest("POST", "/", f)
	require.NoError(t, err)

	fi, err := f.Stat()
	require.NoError(t, err)
	req.Header.Set("Content-Length", strconv.FormatInt(fi.Size(), 10))

	transferReq, err := CreateHTTPUploadRequest(
		HTTPTransferRequest{
			Src: Target{
				Path: "source",
			},
			Dst: Target{
				Path: dst,
			},
			HTTPRequest: req,
		},
	)
	require.NoError(t, err)
	transferReq.dstFS = &localFS{}

	err = TransferFiles(t.Context(), transferReq)
	require.NoError(t, err)

	srcContents, err := os.ReadFile(src)
	require.NoError(t, err)
	dstContents, err := os.ReadFile(dst)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(string(srcContents), string(dstContents)))
}

func TestHTTPDownload(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	src := filepath.Join(tempDir, "source")

	createFile(t, src)
	f, err := os.Open(src)
	require.NoError(t, err)
	t.Cleanup(func() {
		f.Close()
	})

	contents, err := os.ReadFile(src)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	transferReq, err := CreateHTTPDownloadRequest(
		HTTPTransferRequest{
			Src: Target{
				Path: src,
			},
			Dst: Target{
				Path: "/home/robots.txt",
			},
			HTTPResponse: w,
		},
	)
	require.NoError(t, err)

	err = TransferFiles(t.Context(), transferReq)
	require.NoError(t, err)

	data, err := io.ReadAll(w.Body)
	require.NoError(t, err)
	contentLengthStr := strconv.Itoa(len(data))

	require.Empty(t, cmp.Diff(string(contents), string(data)))
	require.Empty(t, cmp.Diff(contentLengthStr, w.Header().Get("Content-Length")))
	require.Empty(t, cmp.Diff("application/octet-stream", w.Header().Get("Content-Type")))
	require.Empty(t, cmp.Diff(`attachment;filename="robots.txt"`, w.Header().Get("Content-Disposition")))
}

func TestTransferUnexpectedLargerFile(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	srcFile := filepath.Join(tempDir, "in")
	require.NoError(t, os.WriteFile(srcFile, []byte("original file data\n"), 0o755))
	dstFile := filepath.Join(tempDir, "out")
	srcFileReader, err := os.Open(srcFile)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, srcFileReader.Close()) })

	srcFS := &mockFS{altData: io.MultiReader(srcFileReader, strings.NewReader("extra data\n"))}
	req := &FileTransferRequest{
		Sources: Sources{
			Paths: []string{srcFile},
		},
		Destination: Target{
			Path: dstFile,
		},
		// Ensure progress bar is created.
		ProgressWriter: io.Discard,
		srcFS:          srcFS,
	}
	require.NoError(t, TransferFiles(t.Context(), req))
	require.FileExists(t, dstFile)
	dstFileData, err := os.ReadFile(dstFile)
	require.NoError(t, err)
	require.Equal(t, "original file data\nextra data\n", string(dstFileData))
}

func createFile(t *testing.T, path string) {
	dir := filepath.Dir(path)
	if dir != path {
		createDir(t, dir)
	}

	// use non-standard permissions to verify that transferred files
	// permissions match the originals
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o654)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, f.Close())
	}()

	// populate file with random amount of random contents
	buf := make([]byte, mathrand.N(fileMaxSize)+1)
	_, err = cryptorand.Read(buf)
	require.NoError(t, err)
	_, err = f.Write(buf)
	require.NoError(t, err)
}

func createDir(t *testing.T, path string) {
	// use non-standard permissions to verify that transferred dirs
	// permissions match the originals
	err := os.MkdirAll(path, 0o765)
	require.NoError(t, err)
}

func checkTransfer(t *testing.T, preserveAttrs bool, dst string, srcs ...string) {
	dstInfo, err := os.Stat(dst)
	require.NoError(t, err)
	if !dstInfo.IsDir() && len(srcs) > 1 {
		t.Fatalf("multiple src files specified, but dst is not a directory")
	}
	// if dst is file, just compare src and dst files
	if !dstInfo.IsDir() {
		compareFiles(t, preserveAttrs, dstInfo, nil, dst, srcs[0])
		return
	}

	for _, src := range srcs {
		srcInfo, err := os.Stat(src)
		require.NoError(t, err)

		// src is file, compare files
		if !srcInfo.IsDir() {
			dstSubPath := filepath.Join(dst, filepath.Base(src))
			dstSubInfo, err := os.Stat(dstSubPath)
			require.NoError(t, err)
			require.False(t, dstSubInfo.IsDir(), "dst file is directory: %q", dstSubPath)
			compareFiles(t, preserveAttrs, dstSubInfo, srcInfo, dstSubPath, src)
			continue
		}

		// src is dir, compare dir trees
		srcDir := filepath.Dir(src)
		err = filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			relPath := strings.TrimPrefix(path, srcDir)
			dstPath := filepath.Join(dst, relPath)
			dstInfo, err := os.Stat(dstPath)
			if err != nil {
				return fmt.Errorf("error getting dst file info: %w", err)
			}
			require.Equal(t, info.IsDir(), dstInfo.IsDir(), "expected %q IsDir=%t, got %t", dstPath, info.IsDir(), dstInfo.IsDir())

			if dstInfo.IsDir() {
				compareFileInfos(t, preserveAttrs, dstInfo, info, dstPath, path)
			} else {
				compareFiles(t, preserveAttrs, dstInfo, info, dstPath, path)
			}

			return nil
		})
		require.NoError(t, err)
	}
}

func compareFiles(t *testing.T, preserveAttrs bool, dstInfo, srcInfo os.FileInfo, dst, src string) {
	var err error
	if srcInfo == nil {
		srcInfo, err = os.Stat(src)
		require.NoError(t, err)
	}

	compareFileInfos(t, preserveAttrs, dstInfo, srcInfo, dst, src)

	dstBytes, err := os.ReadFile(dst)
	require.NoError(t, err)
	srcBytes, err := os.ReadFile(src)
	require.NoError(t, err)
	require.True(t, bytes.Equal(dstBytes, srcBytes), "%q and %q contents not equal", dst, src)
}

func compareFileInfos(t *testing.T, preserveAttrs bool, dstInfo, srcInfo os.FileInfo, dst, src string) {
	require.Equal(t, dstInfo.Size(), srcInfo.Size(), "%q and %q sizes not equal", dst, src)
	require.Equal(t, dstInfo.Mode(), srcInfo.Mode(), "%q and %q perms not equal", dst, src)

	if preserveAttrs {
		require.True(t, dstInfo.ModTime().Equal(srcInfo.ModTime()), "%q and %q mod times not equal", dst, src)
		// don't check access times, locally they line up but they are
		// often different when run in CI
	}
}
