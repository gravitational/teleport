package sshutils

import (
	"fmt"
	"io"

	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
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

func (u *Upstream) Close() error {
	return CloseAll(u.session, u.client)
}

func (m *Upstream) String() string {
	return fmt.Sprintf("upstream(addr=%v)", m.addr)
}

func (u *Upstream) PipeCommand(ch ssh.Channel, command string) (int, error) {
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
	// TODO(klizhentas) close stdin/stdout ?
	go io.Copy(stdin, ch)
	go io.Copy(ch.Stderr(), stderr)
	go io.Copy(ch, stdout)
	err = u.session.Start(command)
	if err != nil {
		return -1, fmt.Errorf("pipe failed to start command %v, err: %v", command, err)
	}
	err = u.session.Wait()
	if err != nil {
		if err, ok := err.(*ssh.ExitError); ok {
			return err.ExitStatus(), nil
		} else {
			return -1, fmt.Errorf("%v failed to wait for ssh command: %v", u, err)
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
	go io.Copy(targetStdin, rw)
	go io.Copy(rw, targetStderr)
	go io.Copy(rw, targetStdout)
	if err := u.session.Shell(); err != nil {
		return err
	}
	return u.session.Wait()
}
