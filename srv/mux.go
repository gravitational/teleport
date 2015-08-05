package srv

import (
	"fmt"
	"io"
	"regexp"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
	"github.com/gravitational/teleport/sshutils"
)

// muxSubsys implements a multiplexing subsystem, in essence it connects to SSH upstreams,
// calls the same exec command on each upstream and sends back all the output from the command
// the muxSubsys uses client's public keys for authentication
type muxSubsys struct {
	query string
	cmd   string
	srv   *Server
}

func parseMuxSubsys(name string) (*muxSubsys, error) {
	out := regexp.MustCompile("mux:([^/]+)/(.+)").FindStringSubmatch(name)
	if len(out) != 3 {
		return nil, fmt.Errorf("invalid format for mux request: '%v', expected 'mux:host:port,host:port/command'", name)
	}
	hs, command := out[1], out[2]
	return &muxSubsys{
		query: hs,
		cmd:   command,
	}, nil
}

func (m *muxSubsys) String() string {
	return fmt.Sprintf("muxSubsys(query=%v, cmd=%v)", m.query, m.cmd)
}

func (m *muxSubsys) execute(sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	log.Infof("%v %v execute()", ctx, m)
	a := ctx.getAgent()
	if a == nil {
		return fmt.Errorf("%v agent forwarding turned off, can not authorize %v", m, a)
	}
	signers, err := a.Signers()
	if err != nil {
		return fmt.Errorf("failed to get signers from agent: %v", err)
	}
	if len(signers) == 0 {
		return fmt.Errorf("no signers in the agent")
	}
	hosts, err := ctx.resolver().resolve(m.query)
	if err != nil {
		log.Infof("%v host resolver failed to resolve query '%v', err: %v", m, m.query, err)
		return err
	}
	mux, err := newMux(m, hosts, sconn.User(), signers)
	if err != nil {
		log.Infof("%v failed to create mux: %v", m, err)
		return err
	}
	// we are registering the closer here, it will be called when server will close the channel
	// because of the end of the operation, or if the client closes connection
	ctx.addCloser(mux)
	code, err := mux.pipe(ctx, ch, m.cmd)
	log.Infof("%v %v collected pipe status: %v %v", ctx, m, code, err)
	ctx.sendResult(execResult{command: m.cmd, code: code})
	return nil
}

func newMux(s *muxSubsys, addrs []string, user string, signers []ssh.Signer) (*mux, error) {
	if len(addrs) == 0 {
		return nil, fmt.Errorf("no upstreams to connect to")
	}
	m := &mux{
		s:   s,
		ups: make(map[string]*sshutils.Upstream),
	}
	for _, a := range addrs {
		up, err := sshutils.DialUpstream(user, a, signers)
		if err != nil {
			log.Infof("%v failed to connect to %v, err: %v", s, a, err)
			continue
		}
		m.ups[a] = up
	}
	if len(m.ups) == 0 {
		return nil, fmt.Errorf("failed connecting to all upstreams")
	}
	return m, nil
}

type mux struct {
	s   *muxSubsys
	ups map[string]*sshutils.Upstream
}

func (m *mux) String() string {
	return m.s.String()
}

func (m *mux) Close() error {
	closers := make([]io.Closer, 0, len(m.ups))
	for _, u := range m.ups {
		closers = append(closers, u)
	}
	return closeAll(closers...)
}

func (m *mux) pipe(ctx *ctx, ch ssh.Channel, command string) (int, error) {
	out := make(chan muxResult, len(m.ups))
	for _, u := range m.ups {
		go func(u *sshutils.Upstream) {
			code, err := u.PipeCommand(ch, command)
			result := muxResult{err: err, code: code, u: u}
			log.Infof("%v %v got %v", ctx, u, &result)
			out <- result
		}(u)
	}
	log.Infof("%v %v will wait for %d results", ctx, m, len(m.ups))
	var lastResult muxResult
	for i := 0; i < len(m.ups); i++ {
		result := <-out
		log.Infof("%v %v collected %v of %d:%d", ctx, m, &result, i, len(m.ups))
	}
	return lastResult.code, lastResult.err
}

type muxResult struct {
	u    *sshutils.Upstream
	err  error
	code int
}

func (m *muxResult) String() string {
	return fmt.Sprintf("muxResult(code=%v, err=%v)", m.code, m.err)
}
