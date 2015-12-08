package teleagent

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/roundtrip"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

func Login(agentAPIAddr string, proxyAddr string, user string,
	password string, hotpToken string,
	ttl time.Duration) error {

	pAgentAPIAddr, err := utils.ParseAddr(agentAPIAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	c, err := roundtrip.NewClient(
		"http://localhost", //domain is not used because of the custom transport
		"v1",
		roundtrip.HTTPClient(
			&http.Client{
				Transport: &http.Transport{
					Dial: func(network, address string) (net.Conn, error) {
						return net.Dial(pAgentAPIAddr.Network, pAgentAPIAddr.Addr)
					}}},
		),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	args := loginArgs{
		ProxyAddr: proxyAddr,
		User:      user,
		Password:  string(password),
		HotpToken: hotpToken,
		TTL:       ttl,
	}

	out, err := c.PostJSON(c.Endpoint("login"), args)
	if err != nil {
		return trace.Wrap(err)
	}

	body := out.Bytes()

	if string(body) == LoginSuccess {
		return nil
	}

	if strings.Contains(string(body), WrongPasswordError) {
		return fmt.Errorf("Wrong user or password or HOTP token")
	}

	return trace.Errorf(string(body))
}

const WrongPasswordError = "ssh: handshake failed: ssh: unable to authenticate, attempted methods [none password], no supported methods remain"
