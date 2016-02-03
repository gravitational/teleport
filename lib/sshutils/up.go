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
package sshutils

import (
	"fmt"
	"io"

	//	log "github.com/Sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

func NewUpstream(clt *ssh.Client) (*Upstream, error) {
	session, err := clt.NewSession()
	if err != nil {
		clt.Close()
		return nil, err
	}
	return &Upstream{
		addr:    clt.Conn.RemoteAddr().String(),
		client:  clt,
		session: session,
	}, nil
}

func DialUpstream(username, addr string, signers []ssh.Signer) (*Upstream, error) {
	sshConfig := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signers...)},
	}
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, err
	}

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, err
	}
	return &Upstream{
		addr:    addr,
		client:  client,
		session: session,
	}, nil
}

type Upstream struct {
	addr    string
	client  *ssh.Client
	session *ssh.Session
}

func (u *Upstream) GetSession() *ssh.Session {
	return u.session
}

func (u *Upstream) Close() error {
	return CloseAll(u.session, u.client)
}

func (m *Upstream) String() string {
	return fmt.Sprintf("upstream(addr=%v)", m.addr)
}

func (u *Upstream) Wait() error {
	return u.session.Wait()
}

func (u *Upstream) CommandRW(command string) (io.ReadWriter, error) {
	stdout, err := u.session.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("fail to pipe stdout: %v", err)
	}
	stdin, err := u.session.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("fail to pipe stdin: %v", err)
	}
	err = u.session.Start(command)
	if err != nil {
		return nil, fmt.Errorf(
			"pipe failed to start command %v, err: %v", command, err)
	}
	return &combo{r: stdout, w: stdin}, nil
}

func (u *Upstream) PipeCommand(ch io.ReadWriter, command string) (int, error) {
	stderr, err := u.session.StderrPipe()
	if err != nil {
		return -1, fmt.Errorf("fail to pipe stderr: %v", err)
	}
	stdout, err := u.session.StdoutPipe()
	if err != nil {
		return -1, fmt.Errorf("fail to pipe stdout: %v", err)
	}
	stdin, err := u.session.StdinPipe()
	if err != nil {
		return -1, fmt.Errorf("fail to pipe stdin: %v", err)
	}
	closeC := make(chan error, 4)

	err = u.session.Start(command)
	if err != nil {
		return -1, fmt.Errorf(
			"pipe failed to start command %v, err: %v", command, err)
	}

	go func() {
		_, err := io.Copy(stdin, ch)
		closeC <- err
	}()

	go func() {
		_, err := io.Copy(ch, stderr)
		closeC <- err
	}()

	go func() {
		_, err := io.Copy(ch, stdout)
		closeC <- err
	}()

	go func() {
		closeC <- u.session.Wait()
	}()

	err = <-closeC
	if err != nil {
		if err, ok := err.(*ssh.ExitError); ok {
			return err.ExitStatus(), nil
		} else {
			return -1, fmt.Errorf(
				"%v failed to wait for ssh command: %v", u, err)
		}
	}
	return 0, nil
}

func (u *Upstream) PipeShellToCh(ch ssh.Channel) error {
	targetStderr, err := u.session.StderrPipe()
	if err != nil {
		return fmt.Errorf("fail to pipe stderr: %v", err)
	}
	targetStdout, err := u.session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("fail to pipe stdout: %v", err)
	}
	targetStdin, err := u.session.StdinPipe()
	if err != nil {
		return fmt.Errorf("fail to pipe stdin: %v", err)
	}
	go io.Copy(targetStdin, ch)
	go io.Copy(ch.Stderr(), targetStderr)
	go io.Copy(ch, targetStdout)
	if err := u.session.Shell(); err != nil {
		return err
	}
	return u.session.Wait()
}

func (u *Upstream) PipeShell(rw io.ReadWriter) error {
	targetStderr, err := u.session.StderrPipe()
	if err != nil {
		return fmt.Errorf("fail to pipe stderr: %v", err)
	}
	targetStdout, err := u.session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("fail to pipe stdout: %v", err)
	}
	targetStdin, err := u.session.StdinPipe()
	if err != nil {
		return fmt.Errorf("fail to pipe stdin: %v", err)
	}
	closeC := make(chan error, 4)

	if err := u.session.Shell(); err != nil {
		return err
	}

	go func() {
		_, err := io.Copy(targetStdin, rw)
		closeC <- err
	}()

	go func() {
		_, err := io.Copy(rw, targetStderr)
		closeC <- err
	}()

	go func() {
		_, err := io.Copy(rw, targetStdout)
		closeC <- err
	}()

	go func() {
		closeC <- u.session.Wait()
	}()

	return <-closeC
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
