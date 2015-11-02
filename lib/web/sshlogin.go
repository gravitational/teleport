package web

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
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

	var res SSHLoginResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, trace.Errorf("error: " + err.Error() + "body: " + string(body))
	}

	if len(res.Err) == 0 {
		return res.Cert, nil
	} else {
		return res.Cert, trace.Errorf(res.Err)
	}
}

type SSHLoginCredentials struct {
	User      string
	Password  string
	HOTPToken string
	PubKey    []byte
	TTL       time.Duration
}

type SSHLoginResponse struct {
	Cert []byte
	Err  string
}

func sshLoginResponse(cert []byte, e error) (jsonResponse []byte) {
	res := SSHLoginResponse{
		Cert: cert,
	}
	if e != nil {
		res.Err = e.Error()
	}
	resJSON, err := json.Marshal(res)
	if err != nil {
		log.Errorf(err.Error())
	}
	return resJSON
}
