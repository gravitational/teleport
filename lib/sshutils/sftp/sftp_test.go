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
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
)

const fileMaxSize = 1000

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

func TestUpload(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		srcPaths        []string
		globbedSrcPaths []string
		dstPath         string
		opts            Options
		files           []string
		errCheck        require.ErrorAssertionFunc
	}{
		{
			name: "one file",
			srcPaths: []string{
				"file",
			},
			dstPath: "copied-file",
			opts: Options{
				PreserveAttrs: true,
			},
			files: []string{
				"file",
			},
		},
		{
			name: "one file to dir",
			srcPaths: []string{
				"file",
			},
			dstPath: "dst/",
			opts: Options{
				PreserveAttrs: true,
			},
			files: []string{
				"file",
				"dst/",
			},
		},
		{
			name: "one dir",
			srcPaths: []string{
				"src/",
			},
			dstPath: "dir/",
			opts: Options{
				PreserveAttrs: true,
				Recursive:     true,
			},
			files: []string{
				"src/",
			},
		},
		{
			name: "two files dst doesn't exist",
			srcPaths: []string{
				"src/file1",
				"src/file2",
			},
			dstPath: "dst/",
			opts: Options{
				PreserveAttrs: true,
			},
			files: []string{
				"src/file1",
				"src/file2",
			},
		},
		{
			name: "two files dst does exist",
			srcPaths: []string{
				"src/file1",
				"src/file2",
			},
			dstPath: "dst/",
			opts: Options{
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
			srcPaths: []string{
				"s",
			},
			dstPath: "dst/",
			opts: Options{
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
			srcPaths: []string{
				"glob*",
			},
			globbedSrcPaths: []string{
				"globS",
				"globA",
				"globT",
				"globB",
			},
			dstPath: "dst/",
			files: []string{
				"globS",
				"globA",
				"globT",
				"globB",
			},
		},
		{
			name: "globbed files dst does exist",
			srcPaths: []string{
				"glob*",
			},
			globbedSrcPaths: []string{
				"globS",
				"globA",
				"globT",
				"globB",
			},
			dstPath: "dst/",
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
			srcPaths: []string{
				"glob*",
				"*stuff",
			},
			globbedSrcPaths: []string{
				"globS",
				"globA",
				"globT",
				"globB",
				"mystuff",
				"yourstuff",
			},
			dstPath: "dst/",
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
			srcPaths: []string{
				"glob*",
				"file",
				"*stuff",
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
			dstPath: "dst/",
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
			name: "recursive glob pattern with normal path",
			srcPaths: []string{
				"glob*",
				"file",
			},
			globbedSrcPaths: []string{
				"globS",
				"globA",
				"globT",
				"globB",
				"globfile",
				"file",
			},
			dstPath: "dst/",
			opts: Options{
				Recursive:     true,
				PreserveAttrs: true,
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
			srcPaths: []string{
				"uno",
				"dos",
				"tres",
			},
			dstPath: "dst_file",
			files: []string{
				"uno",
				"dos",
				"tres",
				"dst_file",
			},
			errCheck: func(t require.TestingT, err error, i ...interface{}) {
				require.EqualError(t, err, fmt.Sprintf(`local file "%s/dst_file" is not a directory, but multiple source files were specified`, i[0]))
			},
		},
		{
			name: "multiple matches from src dst not dir",
			srcPaths: []string{
				"glob*",
			},
			dstPath: "dst_file",
			files: []string{
				"glob1",
				"glob2",
				"glob3",
				"dst_file",
			},
			errCheck: func(t require.TestingT, err error, i ...interface{}) {
				require.EqualError(t, err, fmt.Sprintf(`local file "%s/dst_file" is not a directory, but multiple source files were matched by a glob pattern`, i[0]))
			},
		},
		{
			name: "src dir with recursive not passed",
			srcPaths: []string{
				"src/",
			},
			dstPath: "dst/",
			files: []string{
				"src/",
			},
			errCheck: func(t require.TestingT, err error, i ...interface{}) {
				require.EqualError(t, err, fmt.Sprintf(`"%s/src" is a directory, but the recursive option was not passed`, i[0]))
				require.ErrorAs(t, err, new(*NonRecursiveDirectoryTransferError))
			},
		},
		{
			name: "non-existent src file",
			srcPaths: []string{
				"idontexist",
			},
			errCheck: func(t require.TestingT, err error, i ...interface{}) {
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
			for i := range tt.srcPaths {
				tt.srcPaths[i] = filepath.Join(tempDir, tt.srcPaths[i])
			}
			for i := range tt.globbedSrcPaths {
				tt.globbedSrcPaths[i] = filepath.Join(tempDir, tt.globbedSrcPaths[i])
			}
			tt.dstPath = filepath.Join(tempDir, tt.dstPath)

			cfg, err := CreateUploadConfig(tt.srcPaths, tt.dstPath, tt.opts)
			require.NoError(t, err)
			// use all local filesystems to avoid SSH overhead
			cfg.dstFS = &localFS{}
			err = cfg.initFS(nil, nil)
			require.NoError(t, err)

			ctx := context.Background()
			err = cfg.transfer(ctx)
			if tt.errCheck == nil {
				require.NoError(t, err)
				srcPaths := tt.srcPaths
				if len(tt.globbedSrcPaths) != 0 {
					srcPaths = tt.globbedSrcPaths
				}
				checkTransfer(t, tt.opts.PreserveAttrs, tt.dstPath, srcPaths...)
			} else {
				tt.errCheck(t, err, tempDir)
			}
		})
	}
}

func TestDownload(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		srcPath         string
		globbedSrcPaths []string
		dstPath         string
		opts            Options
		files           []string
		errCheck        require.ErrorAssertionFunc
	}{
		{
			name:    "one file",
			srcPath: "file",
			dstPath: "copied-file",
			opts: Options{
				PreserveAttrs: true,
			},
			files: []string{
				"file",
			},
		},
		{
			name:    "one dir",
			srcPath: "src/",
			dstPath: "dst/",
			opts: Options{
				PreserveAttrs: true,
				Recursive:     true,
			},
			files: []string{
				"src/",
				"dst/",
			},
		},
		{
			name:    "nested dirs",
			srcPath: "s",
			dstPath: "dst/",
			opts: Options{
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
			name:    "globbed files dst doesn't exist",
			srcPath: "glob*",
			globbedSrcPaths: []string{
				"globS",
				"globA",
				"globT",
				"globB",
			},
			dstPath: "dst/",
			files: []string{
				"globS",
				"globA",
				"globT",
				"globB",
			},
		},
		{
			name:    "globbed files dst does exist",
			srcPath: "glob*",
			globbedSrcPaths: []string{
				"globS",
				"globA",
				"globT",
				"globB",
			},
			dstPath: "dst/",
			files: []string{
				"globS",
				"globA",
				"globT",
				"globB",
				"dst/",
			},
		},
		{
			name:    "recursive glob pattern",
			srcPath: "glob*",
			globbedSrcPaths: []string{
				"globS",
				"globA",
				"globT",
				"globB",
				"globfile",
			},
			dstPath: "dst/",
			opts: Options{
				Recursive:     true,
				PreserveAttrs: true,
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
			name:    "src dir with recursive not passed",
			srcPath: "src/",
			dstPath: "dst/",
			files: []string{
				"src/",
			},
			errCheck: func(t require.TestingT, err error, i ...interface{}) {
				require.EqualError(t, err, fmt.Sprintf(`"%s/src" is a directory, but the recursive option was not passed`, i[0]))
			},
		},
		{
			name:    "non-existent src file",
			srcPath: "idontexist",
			errCheck: func(t require.TestingT, err error, i ...interface{}) {
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
			tt.srcPath = filepath.Join(tempDir, tt.srcPath)
			for i := range tt.globbedSrcPaths {
				tt.globbedSrcPaths[i] = filepath.Join(tempDir, tt.globbedSrcPaths[i])
			}
			tt.dstPath = filepath.Join(tempDir, tt.dstPath)

			cfg, err := CreateDownloadConfig(tt.srcPath, tt.dstPath, tt.opts)
			require.NoError(t, err)
			// use all local filesystems to avoid SSH overhead
			cfg.srcFS = &localFS{}
			err = cfg.initFS(nil, nil)
			require.NoError(t, err)

			ctx := context.Background()
			err = cfg.transfer(ctx)
			if tt.errCheck == nil {
				require.NoError(t, err)
				srcPaths := []string{tt.srcPath}
				if len(tt.globbedSrcPaths) != 0 {
					srcPaths = tt.globbedSrcPaths
				}
				checkTransfer(t, tt.opts.PreserveAttrs, tt.dstPath, srcPaths...)
			} else {
				tt.errCheck(t, err, tempDir)
			}
		})
	}
}

func TestHomeDirExpansion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		path         string
		expandedPath string
		errCheck     require.ErrorAssertionFunc
	}{
		{
			name:         "absolute path",
			path:         "/foo/bar",
			expandedPath: "/foo/bar",
		},
		{
			name:         "path with tilde-slash",
			path:         "~/foo/bar",
			expandedPath: "foo/bar",
		},
		{
			name:         "just tilde",
			path:         "~",
			expandedPath: ".",
		},

		{
			name: "~user path",
			path: "~user/foo",
			errCheck: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, PathExpansionError{path: "~user/foo"})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expanded, err := expandPath(tt.path)
			if tt.errCheck == nil {
				require.NoError(t, err)
				require.Equal(t, tt.expandedPath, expanded)
			} else {
				tt.errCheck(t, err)
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
	cfg, err := CreateDownloadConfig(linkPath, dstPath, Options{})
	require.NoError(t, err)
	// use all local filesystems to avoid SSH overhead
	cfg.srcFS = &localFS{}
	err = cfg.initFS(nil, nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = cfg.transfer(ctx)
	require.NoError(t, err)

	checkTransfer(t, false, dstPath, linkPath)
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

	cfg, err := CreateHTTPUploadConfig(
		HTTPTransferRequest{
			Src:         "source",
			Dst:         dst,
			HTTPRequest: req,
		},
	)
	require.NoError(t, err)
	cfg.dstFS = &localFS{}

	err = cfg.transfer(req.Context())
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
	cfg, err := CreateHTTPDownloadConfig(
		HTTPTransferRequest{
			Src:          src,
			Dst:          "/home/robots.txt",
			HTTPResponse: w,
		},
	)
	require.NoError(t, err)
	cfg.srcFS = &localFS{}

	err = cfg.transfer(context.Background())
	require.NoError(t, err)

	data, err := io.ReadAll(w.Body)
	require.NoError(t, err)
	contentLengthStr := strconv.Itoa(len(data))

	require.Empty(t, cmp.Diff(string(contents), string(data)))
	require.Empty(t, cmp.Diff(contentLengthStr, w.Header().Get("Content-Length")))
	require.Empty(t, cmp.Diff("application/octet-stream", w.Header().Get("Content-Type")))
	require.Empty(t, cmp.Diff(`attachment;filename="robots.txt"`, w.Header().Get("Content-Disposition")))
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
