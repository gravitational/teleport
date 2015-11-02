package teleagent

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	"github.com/gravitational/teleport/lib/utils"
)

func Login(agentAddr string, proxyAddr string, user string,
	password string, hotpToken string,
	ttl time.Duration) error {

	pAgentAddr, err := utils.ParseAddr(agentAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	c := &http.Client{
		Transport: &http.Transport{
			Dial: func(network, address string) (net.Conn, error) {
				return net.Dial(pAgentAddr.Network, pAgentAddr.Addr)
			}}}

	ttlJSON, err := json.Marshal(ttl)
	if err != nil {
		return trace.Wrap(err)
	}

	out, err := c.PostForm(
		"http://localhost/login", //domain is not used because of the custom transport
		url.Values{
			"proxyAddr": []string{proxyAddr},
			"user":      []string{user},
			"password":  []string{string(password)},
			"hotpToken": []string{hotpToken},
			"ttl":       []string{string(ttlJSON)},
		})
	if err != nil {
		return trace.Wrap(err)
	}
	defer out.Body.Close()

	body, err := ioutil.ReadAll(out.Body)
	if err != nil {
		return trace.Wrap(err)
	}

	if string(body) == LoginSuccess {
		return nil
	}

	if strings.Contains(string(body), WrongPasswordError) {
		return fmt.Errorf("Wrong user or password or HOTP token")
	}

	return fmt.Errorf(string(body))
}

const WrongPasswordError = "ssh: handshake failed: ssh: unable to authenticate, attempted methods [none password], no supported methods remain"
