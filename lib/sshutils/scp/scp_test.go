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
	log "github.com/sirupsen/logrus"
	. "gopkg.in/check.v1"
)

func TestSCP(t *testing.T) { TestingT(t) }

type SCPSuite struct {
}

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
	s.runSCP(c, cmd, "scp", "-v", "-t", outdir)
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

	s.runSCP(c, cmd, "scp", "-v", "-f", source)

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
	s.runSCP(c, cmd, "scp", "-v", "-t", outDir)

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

	s.runSCP(c, cmd, "scp", "-v", "-f", source)

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
	s.runSCP(c, cmd, "scp", "-v", "-r", "-t", outDir)

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

	s.runSCP(c, cmd, "scp", "-v", "-r", "-f", dir)
	time.Sleep(time.Millisecond * 300)

	name := filepath.Base(dir)
	bytes, err := ioutil.ReadFile(filepath.Join(outDir, name, "target_dir", "target1"))
	c.Assert(err, IsNil)
	c.Assert(string(bytes), Equals, string("file 1"))

	bytes, err = ioutil.ReadFile(filepath.Join(outDir, name, "target2"))
	c.Assert(err, IsNil)
	c.Assert(string(bytes), Equals, string("file 2"))
}

func (s *SCPSuite) TestSCPParsing(c *C) {
	user, host, dest := ParseSCPDestination("root@remote.host:/etc/nginx.conf")
	c.Assert(user, Equals, "root")
	c.Assert(host, Equals, "remote.host")
	c.Assert(dest, Equals, "/etc/nginx.conf")

	user, host, dest = ParseSCPDestination("remote.host:/etc/nginx.co:nf")
	c.Assert(user, Equals, "")
	c.Assert(host, Equals, "remote.host")
	c.Assert(dest, Equals, "/etc/nginx.co:nf")

	user, host, dest = ParseSCPDestination("remote.host:")
	c.Assert(user, Equals, "")
	c.Assert(host, Equals, "remote.host")
	c.Assert(dest, Equals, ".")
}

func (s *SCPSuite) runSCP(c *C, cmd Command, name string, args ...string) {
	scp, in, out, _ := run(name, args...)
	errC := make(chan error, 2)
	successC := make(chan bool)
	rw := &combo{out, in}
	go func() {
		if err := scp.Start(); err != nil {
			errC <- trace.Wrap(err)
		}
		if err := cmd.Execute(rw); err != nil {
			errC <- trace.Wrap(err)
		}
		in.Close()
		if err := scp.Wait(); err != nil {
			errC <- trace.Wrap(err)
		}
		log.Infof("run completed")
		close(successC)
	}()

	select {
	case <-time.After(2 * time.Second):
		c.Fatalf("timeout")
	case err := <-errC:
		c.Assert(err, IsNil)
	case <-successC:
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
