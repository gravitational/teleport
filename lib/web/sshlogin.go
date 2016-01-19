package web

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
)

func SSHAgentLogin(proxyAddr, user, password, hotpToken string, pubKey []byte,
	ttl time.Duration) (cert []byte, err error) {

	cred := SSHLoginCredentials{
		User:      user,
		Password:  password,
		HOTPToken: hotpToken,
		PubKey:    pubKey,
		TTL:       ttl,
	}

	credJSON, err := json.Marshal(cred)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !strings.HasPrefix(proxyAddr, "http://") {
		proxyAddr = "http://" + proxyAddr
	}

	out, err := http.PostForm(
		proxyAddr+"/sshlogin",
		url.Values{
			"credentials": []string{string(credJSON)},
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	defer out.Body.Close()
	body, err := ioutil.ReadAll(out.Body)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if out.StatusCode != 200 {
		return nil, trace.Errorf(string(body))
	}

	return body, nil
}

type SSHLoginCredentials struct {
	User      string
	Password  string
	HOTPToken string
	PubKey    []byte
	TTL       time.Duration
}
