package session

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
)

type SessionCookie struct {
	User string `json:"user"`
	SID  string `json:"sid"`
}

func EncodeCookie(user, sid string) (string, error) {
	bytes, err := json.Marshal(SessionCookie{User: user, SID: sid})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func DecodeCookie(b string) (*SessionCookie, error) {
	bytes, err := hex.DecodeString(b)
	if err != nil {
		return nil, err
	}
	var c *SessionCookie
	if err := json.Unmarshal(bytes, &c); err != nil {
		return nil, err
	}
	return c, nil
}

func SetSession(w http.ResponseWriter, fqdn, user, sid string) error {
	d, err := EncodeCookie(user, sid)
	if err != nil {
		return err
	}
	c := &http.Cookie{
		Domain: fmt.Sprintf(".%v", fqdn),
		Name:   "session",
		Value:  d,
		Path:   "/",
	}
	http.SetCookie(w, c)
	return nil
}

func ClearSession(w http.ResponseWriter, fqdn string) error {
	http.SetCookie(w, &http.Cookie{
		Domain: fmt.Sprintf(".%v", fqdn),
		Name:   "session",
		Value:  "",
		Path:   "/",
	})
	return nil
}
