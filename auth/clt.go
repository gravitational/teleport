package auth

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/gravitational/teleport/backend"
	"github.com/gravitational/teleport/utils"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/roundtrip"
)

const CurrentVersion = "v1"

type Client struct {
	roundtrip.Client
}

func NewClientFromNetAddr(a utils.NetAddr, params ...roundtrip.ClientParam) (*Client, error) {
	client := &http.Client{
		Transport: &http.Transport{
			Dial: func(network, address string) (net.Conn, error) {
				return net.Dial(a.Network, a.Addr)
			}}}
	params = append(params, roundtrip.HTTPClient(client))
	return NewClient("http://placeholder:0", params...)
}

func NewClient(addr string, params ...roundtrip.ClientParam) (*Client, error) {
	c, err := roundtrip.NewClient(addr, CurrentVersion, params...)
	if err != nil {
		return nil, err
	}
	return &Client{*c}, nil
}

func (c *Client) convertResponse(re *roundtrip.Response, err error) (*roundtrip.Response, error) {
	if err != nil {
		return nil, err
	}
	if re.Code() == http.StatusNotFound {
		return nil, &backend.NotFoundError{Message: string(re.Bytes())}
	}
	if re.Code() < 200 || re.Code() > 299 {
		return nil, fmt.Errorf("error: %v", string(re.Bytes()))
	}
	return re, nil
}

func (c *Client) PostForm(endpoint string, vals url.Values) (*roundtrip.Response, error) {
	return c.convertResponse(c.Client.PostForm(endpoint, vals))
}

func (c *Client) Get(u string, params url.Values) (*roundtrip.Response, error) {
	return c.convertResponse(c.Client.Get(u, params))
}

func (c *Client) Delete(u string) (*roundtrip.Response, error) {
	return c.convertResponse(c.Client.Delete(u))
}

func (c *Client) GenerateToken(fqdn string, ttl time.Duration) (string, error) {
	out, err := c.PostForm(c.Endpoint("tokens"), url.Values{
		"fqdn": []string{fqdn},
		"ttl":  []string{ttl.String()},
	})
	if err != nil {
		return "", err
	}
	var re *tokenResponse
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return "", err
	}
	return re.Token, nil
}

func (c *Client) UpsertServer(s backend.Server, ttl time.Duration) error {
	_, err := c.PostForm(c.Endpoint("servers"), url.Values{
		"id":   []string{string(s.ID)},
		"addr": []string{string(s.Addr)},
		"ttl":  []string{ttl.String()},
	})
	return err
}

func (c *Client) GetServers() ([]backend.Server, error) {
	out, err := c.Get(c.Endpoint("servers"), url.Values{})
	if err != nil {
		return nil, err
	}
	var re *serversResponse
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return nil, err
	}
	return re.Servers, nil
}

func (c *Client) UpsertWebTun(wt backend.WebTun, ttl time.Duration) error {
	_, err := c.PostForm(c.Endpoint("tunnels", "web"), url.Values{
		"target": []string{string(wt.TargetAddr)},
		"proxy":  []string{string(wt.ProxyAddr)},
		"prefix": []string{string(wt.Prefix)},
		"ttl":    []string{ttl.String()},
	})
	return err
}

func (c *Client) GetWebTuns() ([]backend.WebTun, error) {
	out, err := c.Get(c.Endpoint("tunnels", "web"), url.Values{})
	if err != nil {
		return nil, err
	}
	var re *webTunsResponse
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return nil, err
	}
	return re.Tunnels, nil
}

func (c *Client) GetWebTun(prefix string) (*backend.WebTun, error) {
	out, err := c.Get(c.Endpoint("tunnels", "web", prefix), url.Values{})
	if err != nil {
		return nil, err
	}
	var re *webTunResponse
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return nil, err
	}
	return &re.Tunnel, nil
}

func (c *Client) DeleteWebTun(prefix string) error {
	_, err := c.Delete(c.Endpoint("tunnels", "web", prefix))
	return err
}

func (c *Client) UpsertPassword(user string, password []byte) error {
	_, err := c.PostForm(
		c.Endpoint("users", user, "web", "password"),
		url.Values{"password": []string{string(password)}},
	)
	return err
}

func (c *Client) CheckPassword(user string, password []byte) error {
	_, err := c.PostForm(
		c.Endpoint("users", user, "web", "password", "check"),
		url.Values{"password": []string{string(password)}})
	return err
}

func (c *Client) SignIn(user string, password []byte) (string, error) {
	out, err := c.PostForm(
		c.Endpoint("users", user, "web", "signin"),
		url.Values{"password": []string{string(password)}},
	)
	if err != nil {
		return "", err
	}
	var re *sessionResponse
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return "", err
	}
	return re.SID, nil
}

func (c *Client) GetWebSession(user string, sid string) (string, error) {
	out, err := c.Get(c.Endpoint("users", user, "web", "sessions", sid), url.Values{})
	if err != nil {
		return "", err
	}
	var re *sessionResponse
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return "", err
	}
	return re.SID, nil
}

func (c *Client) GetWebSessionsKeys(user string) ([]backend.AuthorizedKey, error) {
	out, err := c.Get(c.Endpoint("users", user, "web", "sessions"), url.Values{})
	if err != nil {
		return nil, err
	}
	var re *sessionsResponse
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return nil, err
	}
	return re.Keys, nil
}

func (c *Client) DeleteWebSession(user string, sid string) error {
	_, err := c.Delete(c.Endpoint("users", user, "web", "sessions", sid))
	return err
}

func (c *Client) GetUsers() ([]string, error) {
	out, err := c.Get(c.Endpoint("users"), url.Values{})
	if err != nil {
		return nil, err
	}
	var users *usersResponse
	if err := json.Unmarshal(out.Bytes(), &users); err != nil {
		return nil, err
	}
	return users.Users, nil
}

func (c *Client) DeleteUser(user string) error {
	_, err := c.Delete(c.Endpoint("users", user))
	return err
}

func (c *Client) UpsertUserKey(username string, key backend.AuthorizedKey, ttl time.Duration) ([]byte, error) {
	out, err := c.PostForm(c.Endpoint("users", username, "keys"), url.Values{
		"key": []string{string(key.Value)},
		"id":  []string{key.ID},
		"ttl": []string{ttl.String()},
	})
	if err != nil {
		return nil, err
	}
	var cert *certResponse
	if err := json.Unmarshal(out.Bytes(), &cert); err != nil {
		return nil, err
	}
	return []byte(cert.Cert), err
}

func (c *Client) GetUserKeys(user string) ([]backend.AuthorizedKey, error) {
	out, err := c.Get(c.Endpoint("users", user, "keys"), url.Values{})
	if err != nil {
		return nil, err
	}
	var keys *pubKeysResponse
	if err := json.Unmarshal(out.Bytes(), &keys); err != nil {
		return nil, err
	}
	return keys.PubKeys, nil
}

func (c *Client) DeleteUserKey(username string, id string) error {
	_, err := c.Delete(c.Endpoint("users", username, "keys", id))
	return err
}

func (c *Client) GetHostCAPub() ([]byte, error) {
	out, err := c.Get(c.Endpoint("ca", "host", "keys", "pub"), url.Values{})
	if err != nil {
		return nil, err
	}
	var pubkey *pubKeyResponse
	if err := json.Unmarshal(out.Bytes(), &pubkey); err != nil {
		return nil, err
	}
	return []byte(pubkey.PubKey), err
}

func (c *Client) GetUserCAPub() ([]byte, error) {
	out, err := c.Get(c.Endpoint("ca", "user", "keys", "pub"), url.Values{})
	if err != nil {
		return nil, err
	}
	var pubkey *pubKeyResponse
	if err := json.Unmarshal(out.Bytes(), &pubkey); err != nil {
		return nil, err
	}
	return []byte(pubkey.PubKey), err
}

func (c *Client) GenerateKeyPair(pass string) ([]byte, []byte, error) {
	out, err := c.PostForm(c.Endpoint("keypair"), url.Values{})
	if err != nil {
		return nil, nil, err
	}
	var kp *keyPairResponse
	if err := json.Unmarshal(out.Bytes(), &kp); err != nil {
		return nil, nil, err
	}
	return kp.PrivKey, []byte(kp.PubKey), err
}

func (c *Client) GenerateHostCert(key []byte, id, hostname string, ttl time.Duration) ([]byte, error) {
	out, err := c.PostForm(c.Endpoint("ca", "host", "certs"), url.Values{
		"key":      []string{string(key)},
		"id":       []string{id},
		"hostname": []string{hostname},
		"ttl":      []string{ttl.String()},
	})
	if err != nil {
		return nil, err
	}
	var cert *certResponse
	if err := json.Unmarshal(out.Bytes(), &cert); err != nil {
		return nil, err
	}
	return []byte(cert.Cert), err
}

func (c *Client) GenerateUserCert(key []byte, id, user string, ttl time.Duration) ([]byte, error) {
	out, err := c.PostForm(c.Endpoint("ca", "user", "certs"), url.Values{
		"key":  []string{string(key)},
		"id":   []string{id},
		"user": []string{user},
		"ttl":  []string{ttl.String()},
	})
	if err != nil {
		return nil, err
	}
	var cert *certResponse
	if err := json.Unmarshal(out.Bytes(), &cert); err != nil {
		return nil, err
	}
	return []byte(cert.Cert), err
}

func (c *Client) ResetHostCA() error {
	_, err := c.PostForm(c.Endpoint("ca", "host", "keys"), url.Values{})
	return err
}

func (c *Client) ResetUserCA() error {
	_, err := c.PostForm(c.Endpoint("ca", "user", "keys"), url.Values{})
	return err
}
