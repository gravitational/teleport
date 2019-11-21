/*
Copyright 2018 Gravitational, Inc.

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
package scp

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	. "gopkg.in/check.v1"
)

func TestSCP(t *testing.T) { TestingT(t) }

type SCPSuite struct {
}

var _ = fmt.Printf
var _ = Suite(&SCPSuite{})

func (s *SCPSuite) SetUpSuite(c *C) {
	utils.InitLoggerForTests()
}

func (s *SCPSuite) TestHTTPSendFile(c *C) {
	outdir := c.MkDir()
	expectedBytes := []byte("hello")
	buf := bytes.NewReader(expectedBytes)
	req, err := http.NewRequest("POST", "/", buf)
	c.Assert(err, IsNil)

	req.Header.Set("Content-Length", strconv.Itoa(len(expectedBytes)))

	stdOut := bytes.NewBufferString("")
	cmd, err := CreateHTTPUpload(
		HTTPTransferRequest{
			FileName:       "filename",
			RemoteLocation: outdir,
			HTTPRequest:    req,
			Progress:       stdOut,
			User:           "test-user",
		})
	c.Assert(err, IsNil)
	err = runSCP(cmd, "scp", "-v", "-t", outdir)
	c.Assert(err, IsNil)
	bytesReceived, err := ioutil.ReadFile(filepath.Join(outdir, "filename"))
	c.Assert(err, IsNil)
	c.Assert(string(bytesReceived), Equals, string(expectedBytes))
}

func (s *SCPSuite) TestHTTPReceiveFile(c *C) {
	dir := c.MkDir()
	source := filepath.Join(dir, "target")

	contents := []byte("hello, file contents!")
	err := ioutil.WriteFile(source, contents, 0666)
	c.Assert(err, IsNil)

	w := httptest.NewRecorder()
	stdOut := bytes.NewBufferString("")
	cmd, err := CreateHTTPDownload(
		HTTPTransferRequest{
			RemoteLocation: "/home/robots.txt",
			HTTPResponse:   w,
			User:           "test-user",
			Progress:       stdOut,
		})

	c.Assert(err, IsNil)

	err = runSCP(cmd, "scp", "-v", "-f", source)
	c.Assert(err, IsNil)

	data, err := ioutil.ReadAll(w.Body)
	contentLengthStr := strconv.Itoa(len(data))
	c.Assert(err, IsNil)
	c.Assert(string(data), Equals, string(contents))
	c.Assert(contentLengthStr, Equals, w.Header().Get("Content-Length"))
	c.Assert("application/octet-stream", Equals, w.Header().Get("Content-Type"))
	c.Assert(`attachment;filename="robots.txt"`, Equals, w.Header().Get("Content-Disposition"))
}

func (s *SCPSuite) TestSendFile(c *C) {
	dir := c.MkDir()
	target := filepath.Join(dir, "target")
	contents := []byte("hello, send file!")

	err := ioutil.WriteFile(target, contents, 0666)
	c.Assert(err, IsNil)

	cmd, err := CreateCommand(
		Config{
			User: "test-user",
			Flags: Flags{
				Source: true,
				Target: []string{target},
			},
		},
	)
	c.Assert(err, IsNil)

	outDir := c.MkDir()
	err = runSCP(cmd, "scp", "-v", "-t", outDir)
	c.Assert(err, IsNil)

	bytes, err := ioutil.ReadFile(filepath.Join(outDir, "target"))
	c.Assert(err, IsNil)
	c.Assert(string(bytes), Equals, string(contents))
}

func (s *SCPSuite) TestReceiveFile(c *C) {
	dir := c.MkDir()
	source := filepath.Join(dir, "target")
	contents := []byte("hello, file contents!")

	err := ioutil.WriteFile(source, contents, 0666)
	c.Assert(err, IsNil)

	outDir := c.MkDir() + "/"
	cmd, err := CreateCommand(Config{
		User: "test-user",
		Flags: Flags{
			Sink:   true,
			Target: []string{outDir},
		},
	})
	c.Assert(err, IsNil)

	err = runSCP(cmd, "scp", "-v", "-f", source)
	c.Assert(err, IsNil)

	bytes, err := ioutil.ReadFile(filepath.Join(outDir, "target"))
	c.Assert(err, IsNil)
	c.Assert(string(bytes), Equals, string(contents))
}

func (s *SCPSuite) TestSendDir(c *C) {
	dir := c.MkDir()
	c.Assert(os.Mkdir(filepath.Join(dir, "target_dir"), 0777), IsNil)

	err := ioutil.WriteFile(
		filepath.Join(dir, "target_dir", "target1"), []byte("file 1"), 0666)
	c.Assert(err, IsNil)

	err = ioutil.WriteFile(
		filepath.Join(dir, "target2"), []byte("file 2"), 0666)
	c.Assert(err, IsNil)

	cmd, err := CreateCommand(Config{
		User: "test-user",
		Flags: Flags{
			Source:    true,
			Target:    []string{dir},
			Recursive: true,
		},
	})
	c.Assert(err, IsNil)

	outDir := c.MkDir()
	err = runSCP(cmd, "scp", "-v", "-r", "-t", outDir)
	c.Assert(err, IsNil)

	name := filepath.Base(dir)
	bytes, err := ioutil.ReadFile(filepath.Join(outDir, name, "target_dir", "target1"))
	c.Assert(err, IsNil)
	c.Assert(string(bytes), Equals, string("file 1"))

	bytes, err = ioutil.ReadFile(filepath.Join(outDir, name, "target2"))
	c.Assert(err, IsNil)
	c.Assert(string(bytes), Equals, string("file 2"))
}

func (s *SCPSuite) TestReceiveDir(c *C) {
	dir := c.MkDir()
	c.Assert(os.Mkdir(filepath.Join(dir, "target_dir"), 0777), IsNil)

	err := ioutil.WriteFile(
		filepath.Join(dir, "target_dir", "target1"), []byte("file 1"), 0666)
	c.Assert(err, IsNil)

	err = ioutil.WriteFile(
		filepath.Join(dir, "target2"), []byte("file 2"), 0666)
	c.Assert(err, IsNil)

	outDir := c.MkDir() + "/"
	cmd, err := CreateCommand(Config{
		User: "test-user",
		Flags: Flags{
			Sink:      true,
			Target:    []string{outDir},
			Recursive: true,
		},
	})
	c.Assert(err, IsNil)

	err = runSCP(cmd, "scp", "-v", "-r", "-f", dir)
	c.Assert(err, IsNil)
	time.Sleep(time.Millisecond * 300)

	name := filepath.Base(dir)
	bytes, err := ioutil.ReadFile(filepath.Join(outDir, name, "target_dir", "target1"))
	c.Assert(err, IsNil)
	c.Assert(string(bytes), Equals, string("file 1"))

	bytes, err = ioutil.ReadFile(filepath.Join(outDir, name, "target2"))
	c.Assert(err, IsNil)
	c.Assert(string(bytes), Equals, string("file 2"))
}

func (s *SCPSuite) TestInvalidDir(c *C) {
	cmd, err := CreateCommand(Config{
		User: "test-user",
		Flags: Flags{
			Sink:      true,
			Target:    []string{},
			Recursive: true,
		},
	})
	c.Assert(err, IsNil)

	tests := []struct {
		inDirName string
	}{
		{inDirName: ""},
		{inDirName: "."},
		{inDirName: ".."},
	}

	for _, tt := range tests {
		scp, in, out, _ := run("scp", "-v", "-r", "-f", tt.inDirName)
		rw := &combo{out, in}

		err := scp.Start()
		c.Assert(err, IsNil)

		err = cmd.Execute(rw)
		c.Assert(err, NotNil)
	}
}

// TestVerifyDir makes sure that if scp was started in directory mode (the
// user attempts to copy multiple files or a directory), the target is a
// directory.
func (s *SCPSuite) TestVerifyDir(c *C) {
	// Create temporary directory with a file "target" in it.
	dir := c.MkDir()
	target := filepath.Join(dir, "target")
	err := ioutil.WriteFile(target, []byte{}, 0666)
	c.Assert(err, IsNil)

	cmd, err := CreateCommand(
		Config{
			User: "test-user",
			Flags: Flags{
				Source: true,
				Target: []string{target},
			},
		},
	)
	c.Assert(err, IsNil)

	// Run command with -d flag (directory mode). Since the target is a file,
	// it should fail.
	err = runSCP(cmd, "scp", "-t", "-d", target)
	c.Assert(err, NotNil)
}

func (s *SCPSuite) TestSCPParsing(c *C) {
	type tc struct {
		in   string
		dest Destination
		err  error
	}
	testCases := []tc{
		{
			in:   "root@remote.host:/etc/nginx.conf",
			dest: Destination{Login: "root", Host: utils.NetAddr{Addr: "remote.host", AddrNetwork: "tcp"}, Path: "/etc/nginx.conf"},
		},
		{
			in:   "remote.host:/etc/nginx.co:nf",
			dest: Destination{Host: utils.NetAddr{Addr: "remote.host", AddrNetwork: "tcp"}, Path: "/etc/nginx.co:nf"},
		},
		{
			in:   "[::1]:/etc/nginx.co:nf",
			dest: Destination{Host: utils.NetAddr{Addr: "[::1]", AddrNetwork: "tcp"}, Path: "/etc/nginx.co:nf"},
		},
		{
			in:   "root@123.123.123.123:/var/www/html/",
			dest: Destination{Login: "root", Host: utils.NetAddr{Addr: "123.123.123.123", AddrNetwork: "tcp"}, Path: "/var/www/html/"},
		},
		{
			in:   "myusername@myremotehost.com:/home/hope/*",
			dest: Destination{Login: "myusername", Host: utils.NetAddr{Addr: "myremotehost.com", AddrNetwork: "tcp"}, Path: "/home/hope/*"},
		},
	}
	for i, tc := range testCases {
		comment := Commentf("Test case %v: %q", i, tc.in)
		re, err := ParseSCPDestination(tc.in)
		if tc.err == nil {
			c.Assert(err, IsNil, comment)
			c.Assert(re.Login, Equals, tc.dest.Login, comment)
			c.Assert(re.Host, DeepEquals, tc.dest.Host, comment)
			c.Assert(re.Path, Equals, tc.dest.Path, comment)
		} else {
			c.Assert(err, FitsTypeOf, tc.err)
		}
	}
}

func runSCP(cmd Command, name string, args ...string) error {
	scp, in, out, _ := run(name, args...)
	rw := &combo{out, in}

	errCh := make(chan error, 1)

	go func() {
		if err := scp.Start(); err != nil {
			errCh <- trace.Wrap(err)
			return
		}
		if err := cmd.Execute(rw); err != nil {
			errCh <- trace.Wrap(err)
			return
		}
		in.Close()
		if err := scp.Wait(); err != nil {
			errCh <- trace.Wrap(err)
			return
		}
		close(errCh)
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

type combo struct {
	r io.Reader
	w io.Writer
}

func (c *combo) Read(b []byte) (int, error) {
	return c.r.Read(b)
}

func (c *combo) Write(b []byte) (int, error) {
	return c.w.Write(b)
}

func run(name string, args ...string) (*exec.Cmd, io.WriteCloser, io.ReadCloser, io.ReadCloser) {
	cmd := exec.Command(name, args...)

	in, err := cmd.StdinPipe()
	if err != nil {
		panic(err)
	}

	out, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}

	epipe, err := cmd.StderrPipe()
	if err != nil {
		panic(err)
	}

	return cmd, in, out, epipe
}
