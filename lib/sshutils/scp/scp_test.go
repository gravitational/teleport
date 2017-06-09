/*
Copyright 2015 Gravitational, Inc.

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
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	. "gopkg.in/check.v1"
)

func TestSCP(t *testing.T) { TestingT(t) }

type SCPSuite struct {
}

var _ = Suite(&SCPSuite{})

func (s *SCPSuite) SetUpSuite(c *C) {
	utils.InitLoggerForTests()
}

func (s *SCPSuite) TestSendFile(c *C) {
	dir := c.MkDir()
	target := filepath.Join(dir, "target")

	contents := []byte("hello, send file!")

	err := ioutil.WriteFile(target, contents, 0666)
	c.Assert(err, IsNil)

	srv := &Command{Source: true, Target: []string{target}}

	outDir := c.MkDir()
	cmd, in, out, _ := command("scp", "-v", "-t", outDir)

	errC := make(chan error, 2)
	successC := make(chan bool)
	rw := &combo{out, in}
	go func() {
		if err := cmd.Start(); err != nil {
			errC <- trace.Wrap(err)
		}
		if err := srv.Execute(rw); err != nil {
			errC <- trace.Wrap(err)
		}
		in.Close()
		if err := cmd.Wait(); err != nil {
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

	srv := &Command{Sink: true, Target: []string{outDir}}

	cmd, in, out, _ := command("scp", "-v", "-f", source)

	errC := make(chan error, 3)
	successC := make(chan bool, 1)
	rw := &combo{out, in}
	// http://stackoverflow.com/questions/20134095/why-do-i-get-bad-file-descriptor-in-this-go-program-using-stderr-and-ioutil-re
	go func() {
		err := cmd.Start()
		if err != nil {
			errC <- trace.Wrap(err)
		}
		log.Infof("serving")
		err = trace.Wrap(srv.Execute(rw))
		if err != nil {
			errC <- err
		}
		in.Close()
		log.Infof("done")
		if err := trace.Wrap(cmd.Wait()); err != nil {
			errC <- err
		}
		log.Infof("run completed")
		successC <- true
	}()

	select {
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for results")
	case err := <-errC:
		c.Assert(err, IsNil)
	case <-successC:
	}

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

	srv := &Command{Source: true, Target: []string{dir}, Recursive: true}

	outDir := c.MkDir()

	cmd, in, out, _ := command("scp", "-v", "-r", "-t", outDir)

	errC := make(chan error, 2)
	successC := make(chan bool)
	rw := &combo{out, in}
	go func() {
		if err := cmd.Start(); err != nil {
			errC <- trace.Wrap(err)
		}
		if err := srv.Execute(rw); err != nil {
			errC <- trace.Wrap(err)
		}
		in.Close()
		if err := cmd.Wait(); err != nil {
			errC <- trace.Wrap(err)
		}
		log.Infof("run completed")
		close(successC)
	}()

	select {
	case <-time.After(time.Second):
		panic("timeout")
	case err := <-errC:
		c.Assert(err, IsNil)
	case <-successC:

	}

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

	srv := &Command{Sink: true, Target: []string{outDir}, Recursive: true}

	cmd, in, out, _ := command("scp", "-v", "-r", "-f", dir)

	errC := make(chan error, 2)
	successC := make(chan bool)
	rw := &combo{out, in}
	go func() {
		if err := cmd.Start(); err != nil {
			errC <- trace.Wrap(err)
		}
		if err := srv.Execute(rw); err != nil {
			errC <- trace.Wrap(err)
		}
		in.Close()
		log.Infof("run completed")
		close(successC)
	}()

	select {
	case <-time.After(time.Second):
		c.Fatalf("timeout")
	case err := <-errC:
		c.Assert(err, IsNil)
	case <-successC:
	}

	time.Sleep(time.Millisecond * 300)

	name := filepath.Base(dir)
	bytes, err := ioutil.ReadFile(filepath.Join(outDir, name, "target_dir", "target1"))
	c.Assert(err, IsNil)
	c.Assert(string(bytes), Equals, string("file 1"))

	bytes, err = ioutil.ReadFile(filepath.Join(outDir, name, "target2"))
	c.Assert(err, IsNil)
	c.Assert(string(bytes), Equals, string("file 2"))
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

func command(name string, args ...string) (*exec.Cmd, io.WriteCloser, io.ReadCloser, io.ReadCloser) {
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
