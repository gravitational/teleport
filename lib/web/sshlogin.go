package web

import (
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"runtime"
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

// SSHAgentOIDCLogin is used by SSH Agent to login using OpenID connect
func SSHAgentOIDCLogin(proxyAddr string, pubKey []byte, ttl time.Duration, insecure bool, pool *x509.CertPool) (*SSHLoginResponse, error) {
	clt, err := initClient(proxyAddr, insecure, pool)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	waitC := make(chan *SSHLoginResponse, 1)
	errorC := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/callback" {
			http.NotFound(w, r)
			return
		}
		out := r.URL.Query().Get("response")
		if out == "" {
			log.Infof("missing required query parameters in %v", r.URL.String())
		}
		var re *SSHLoginResponse
		log.Infof("callback got response: %v", r.URL.String())
		err := json.Unmarshal([]byte(out), &re)
		if err != nil {
			log.Infof("failed to unmarshal response: %v", err)
			errorC <- trace.Wrap(err)
			fmt.Fprintf(w, "failed to login")
			return
		}
		waitC <- re
		fmt.Fprintf(w, "logged in")
	}))
	defer server.Close()

	out, err := clt.PostJSON(clt.Endpoint("webapi", "oidc", "login", "console"), oidcLoginConsoleReq{
		RedirectURL: server.URL + "/callback",
		PublicKey:   pubKey,
		CertTTL:     ttl,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var re *oidcLoginConsoleResponse
	err = json.Unmarshal(out.Bytes(), &re)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	fmt.Printf("If browser window does not open automatically, open it by clicking on the link:\n %v\n", re.RedirectURL)

	var command = "sensible-browser"
	if runtime.GOOS == "darwin" {
		command = "open"
	}
	path, err := exec.LookPath(command)
	if err == nil {
		exec.Command(path, re.RedirectURL).Start()
	}

	log.Infof("waiting for response on %v", server.URL)

	select {
	case err := <-errorC:
		log.Infof("got error: ", err)
		return nil, trace.Wrap(err)
	case response := <-waitC:
		log.Infof("got response: ", err)
		return response, nil
	case <-time.After(60 * time.Second):
		log.Infof("got timeout ")
		return nil, trace.Wrap(trace.Errorf("timeout waiting for callback"))
	}

}

// SSHAgentLogin issues call to web proxy and receives temp certificate
// if credentials are valid
//
// proxyAddr must be specified as host:port
func SSHAgentLogin(proxyAddr, user, password, hotpToken string, pubKey []byte, ttl time.Duration, insecure bool, pool *x509.CertPool) (*SSHLoginResponse, error) {
	clt, err := initClient(proxyAddr, insecure, pool)
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

func initClient(proxyAddr string, insecure bool, pool *x509.CertPool) (*webClient, error) {
	// validate proxyAddr:
	host, port, err := net.SplitHostPort(proxyAddr)
	if err != nil || host == "" || port == "" {
		if err != nil {
			log.Error(err)
		}
		return nil, trace.Wrap(
			teleport.BadParameter("proxyAddress",
				fmt.Sprintf("'%v' is not a valid proxy address", proxyAddr)))
	}
	proxyAddr = "https://" + net.JoinHostPort(host, port)

	var opts []roundtrip.ClientParam

	if pool != nil {
		// use custom set of trusted CAs
		opts = append(opts, roundtrip.HTTPClient(newClientWithPool(pool)))
	} else if insecure {
		// skip https cert verification, oh no!
		fmt.Printf("WARNING: You are using insecure connection to SSH proxy %v\n", proxyAddr)
		opts = append(opts, roundtrip.HTTPClient(newInsecureClient()))
	}

	clt, err := newWebClient(proxyAddr, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return clt, nil
}
