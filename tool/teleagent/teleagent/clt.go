package teleagent

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	"github.com/gravitational/teleport/lib/utils"
)

func Login(agentAddr string, proxyAddr string, user string,
	password string, hotpToken string, ttl time.Duration) error {

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

	_, err = c.PostForm(
		"http://domainnotused/login",
		url.Values{
			"proxyAddr": []string{proxyAddr},
			"user":      []string{user},
			"password":  []string{string(password)},
			"hotpToken": []string{hotpToken},
			"ttl":       []string{string(ttlJSON)},
		})
	return err
}
