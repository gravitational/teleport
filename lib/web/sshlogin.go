package web

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/gravitational/teleport"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
)

const (
	// HTTPS is https prefix
	HTTPS = "https"
	// WSS is secure web sockets prefix
	WSS = "wss"
)

// SSHAgentLogin issues call to web proxy and receives temp certificate
// if credentials are valid
func SSHAgentLogin(proxyAddr, user, password, hotpToken string, pubKey []byte, ttl time.Duration, insecure bool) (*SSHLoginResponse, error) {

	u, err := url.Parse(proxyAddr)
	if err != nil {
		return nil, trace.Wrap(
			teleport.BadParameter("proxyAddress",
				fmt.Sprintf("'%v' is not a valid URL", proxyAddr)))
	}
	u.Scheme = HTTPS
	proxyAddr = u.String()

	var opts []roundtrip.ClientParam
	if insecure {
		log.Warningf("you are using insecure HTTPS connection")
		opts = append(opts, roundtrip.HTTPClient(newInsecureClient()))
	}

	clt, err := newWebClient(proxyAddr, opts...)
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
