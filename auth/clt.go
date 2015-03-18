package auth

import (
	"encoding/json"
	"fmt"
	"time"

	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/gravitational/teleport/backend"
)

const CurrentVersion = "v1"

type Client struct {
	addr   string
	client *http.Client
}

func NewClient(addr string) *Client {
	return &Client{
		addr:   addr,
		client: http.DefaultClient,
	}
}

func (c *Client) GetUsers() ([]string, error) {
	out, err := c.Get(c.endpoint("users"), url.Values{})
	if err != nil {
		return nil, err
	}
	var users *usersResponse
	if err := json.Unmarshal(out, &users); err != nil {
		return nil, err
	}
	return users.Users, nil
}

func (c *Client) DeleteUser(user string) error {
	return c.Delete(c.endpoint("users", user))
}

func (c *Client) UpsertUserKey(username string, key backend.AuthorizedKey, ttl time.Duration) ([]byte, error) {
	out, err := c.PostForm(c.endpoint("users", username, "keys"), url.Values{
		"key": []string{string(key.Value)},
		"id":  []string{key.ID},
		"ttl": []string{ttl.String()},
	})
	if err != nil {
		return nil, err
	}
	var cert *certResponse
	if err := json.Unmarshal(out, &cert); err != nil {
		return nil, err
	}
	return []byte(cert.Cert), err
}

func (c *Client) GetUserKeys(user string) ([]backend.AuthorizedKey, error) {
	out, err := c.Get(c.endpoint("users", user, "keys"), url.Values{})
	if err != nil {
		return nil, err
	}
	var keys *pubKeysResponse
	if err := json.Unmarshal(out, &keys); err != nil {
		return nil, err
	}
	return keys.PubKeys, nil
}

func (c *Client) DeleteUserKey(username string, id string) error {
	return c.Delete(c.endpoint("users", username, "keys", id))
}

func (c *Client) GetHostCAPub() ([]byte, error) {
	out, err := c.Get(c.endpoint("ca", "host", "keys", "pub"), url.Values{})
	if err != nil {
		return nil, err
	}
	var pubkey *pubKeyResponse
	if err := json.Unmarshal(out, &pubkey); err != nil {
		return nil, err
	}
	return []byte(pubkey.PubKey), err
}

func (c *Client) GetUserCAPub() ([]byte, error) {
	out, err := c.Get(c.endpoint("ca", "user", "keys", "pub"), url.Values{})
	if err != nil {
		return nil, err
	}
	var pubkey *pubKeyResponse
	if err := json.Unmarshal(out, &pubkey); err != nil {
		return nil, err
	}
	return []byte(pubkey.PubKey), err
}

func (c *Client) GenerateKeyPair(pass string) ([]byte, []byte, error) {
	out, err := c.PostForm(c.endpoint("keypair"), url.Values{})
	if err != nil {
		return nil, nil, err
	}
	var kp *keyPairResponse
	if err := json.Unmarshal(out, &kp); err != nil {
		return nil, nil, err
	}
	return kp.PrivKey, []byte(kp.PubKey), err
}

func (c *Client) GenerateHostCert(key []byte, id, hostname string, ttl time.Duration) ([]byte, error) {
	out, err := c.PostForm(c.endpoint("ca", "host", "certs"), url.Values{
		"key":      []string{string(key)},
		"id":       []string{id},
		"hostname": []string{hostname},
		"ttl":      []string{ttl.String()},
	})
	if err != nil {
		return nil, err
	}
	var cert *certResponse
	if err := json.Unmarshal(out, &cert); err != nil {
		return nil, err
	}
	return []byte(cert.Cert), err
}

func (c *Client) GenerateUserCert(key []byte, id, user string, ttl time.Duration) ([]byte, error) {
	out, err := c.PostForm(c.endpoint("ca", "user", "certs"), url.Values{
		"key":  []string{string(key)},
		"id":   []string{id},
		"user": []string{user},
		"ttl":  []string{ttl.String()},
	})
	if err != nil {
		return nil, err
	}
	var cert *certResponse
	if err := json.Unmarshal(out, &cert); err != nil {
		return nil, err
	}
	return []byte(cert.Cert), err
}

func (c *Client) ResetHostCA() error {
	_, err := c.PostForm(c.endpoint("ca", "host", "keys"), url.Values{})
	return err
}

func (c *Client) ResetUserCA() error {
	_, err := c.PostForm(c.endpoint("ca", "user", "keys"), url.Values{})
	return err
}

func (c *Client) PostForm(endpoint string, vals url.Values) ([]byte, error) {
	return c.RoundTrip(func() (*http.Response, error) {
		return c.client.Post(
			endpoint, "application/x-www-form-urlencoded",
			strings.NewReader(vals.Encode()))
	})
}

func (c *Client) Delete(endpoint string) error {
	data, err := c.RoundTrip(func() (*http.Response, error) {
		req, err := http.NewRequest("DELETE", endpoint, nil)
		if err != nil {
			return nil, err
		}
		return c.client.Do(req)
	})
	if err != nil {
		return err
	}
	var re *StatusResponse
	err = json.Unmarshal(data, &re)
	return err
}

func (c *Client) Get(u string, params url.Values) ([]byte, error) {
	baseUrl, err := url.Parse(u)
	if err != nil {
		return nil, err
	}
	baseUrl.RawQuery = params.Encode()
	return c.RoundTrip(func() (*http.Response, error) {
		return c.client.Get(baseUrl.String())
	})
}

type RoundTripFn func() (*http.Response, error)

func (c *Client) RoundTrip(fn RoundTripFn) ([]byte, error) {
	re, err := fn()
	if err != nil {
		return nil, err
	}
	defer re.Body.Close()
	body, err := ioutil.ReadAll(re.Body)
	if err != nil {
		return nil, err
	}
	if re.StatusCode != http.StatusOK && re.StatusCode != http.StatusCreated {
		var s *StatusResponse
		if err := json.Unmarshal(body, &s); err != nil {
			return nil, fmt.Errorf(
				"failed to decode response '%s', error: %v", string(body), err)
		}
		s.StatusCode = re.StatusCode
		return nil, s
	}
	return body, nil
}

func (c *Client) endpoint(params ...string) string {
	return fmt.Sprintf("%s/%s/%s", c.addr, CurrentVersion, strings.Join(params, "/"))
}

type StatusResponse struct {
	StatusCode int
	Message    string `json:"message"`
}

func (e *StatusResponse) Error() string {
	return e.Message
}
