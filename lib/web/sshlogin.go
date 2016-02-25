package web

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/gravitational/trace"
)

// SSHAgentLogin issues call to web proxy and receives temp certificate
// in case if credentials are valid
func SSHAgentLogin(proxyAddr, user, password, hotpToken string, pubKey []byte, ttl time.Duration) (*SSHLoginResponse, error) {

	// TODO(klizhentas) HTTPS of course
	if !strings.HasPrefix(proxyAddr, "http://") {
		proxyAddr = "http://" + proxyAddr
	}

	clt, err := newWebClient(proxyAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	re, err := clt.PostJSON(clt.Endpoint("webapi", "ssh", "certs"), createSSHCertReq{
		User:      user,
		Password:  password,
		HOTPToken: hotpToken,
		PubKey:    pubKey,
		TTL:       ttl,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var out *SSHLoginResponse
	err = json.Unmarshal(re.Bytes(), &out)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return out, nil
}
