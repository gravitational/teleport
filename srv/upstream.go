package srv

import (
	"fmt"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/log"
	"io"

	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
)

func connectUpstream(username, addr string, signers []ssh.Signer) (*upstream, error) {
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
	return &upstream{
		addr:    addr,
		client:  client,
		session: session,
	}, nil
}

type upstream struct {
	addr    string
	client  *ssh.Client
	session *ssh.Session
}

func (u *upstream) Close() error {
	return closeAll(u.session, u.client)
}

func (m *upstream) String() string {
	return fmt.Sprintf("upstream(addr=%v)", m.addr)
}

func (u *upstream) pipe(ctx *ctx, ch ssh.Channel, command string) (int, error) {
	log.Infof("%v %v pipe start", ctx, u)
	stderr, err := u.session.StderrPipe()
	if err != nil {
		return -1, fmt.Errorf("%v fail to pipe stderr: %v", ctx, err)
	}
	stdout, err := u.session.StdoutPipe()
	if err != nil {
		return -1, fmt.Errorf("%v fail to pipe stdout: %v", ctx, err)
	}
	stdin, err := u.session.StdinPipe()
	if err != nil {
		return -1, fmt.Errorf("%v fail to pipe stdin: %v", ctx, err)
	}
	// TODO(klizhentas) close stdin/stdout ?
	go io.Copy(stdin, ch)
	go io.Copy(ch.Stderr(), stderr)
	go io.Copy(ch, stdout)
	err = u.session.Start(command)
	if err != nil {
		log.Infof("%v pipe failed to start command %v, err: %v", ctx, command, err)
		return -1, fmt.Errorf("%v pipe failed to start command %v, err: %v", ctx, command, err)
	}
	log.Infof("%v %v pipe: session.Wait()", ctx, u)
	err = u.session.Wait()
	log.Infof("%v %v pipe: session.Wait() completed", ctx, u)
	if err != nil {
		log.Infof("%v %v got error: %v %T", ctx, u, err, err)
		if err, ok := err.(*ssh.ExitError); ok {
			log.Infof("%v %v got exit error: %v", ctx, u, err)
			return err.ExitStatus(), nil
		} else {
			return -1, fmt.Errorf("%v %v failed to wait for ssh command: %v", ctx, u, err)
		}
	}
	return 0, nil
}

func (u *upstream) pipeShell(ctx *ctx, ch ssh.Channel) error {
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
		log.Infof("Failed to start shell: %v", err)
		return err
	}
	log.Infof("session started successfully")
	return u.session.Wait()
}
