package idemeumjwt

import (
	"context"
	"crypto/ecdsa"
	"crypto/subtle"
	"fmt"
	"github.com/form3tech-oss/jwt-go"
	"github.com/gravitational/trace"
	"golang.org/x/oauth2"
	"gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/json"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

var TimeFunc = time.Now

type IdemeumClaims struct {
	Audience   string   `json:"aud,omitempty"`
	ExpiresAt  int64    `json:"exp,omitempty"`
	Id         string   `json:"jti,omitempty"`
	IssuedAt   int64    `json:"iat,omitempty"`
	Issuer     string   `json:"iss,omitempty"`
	NotBefore  int64    `json:"nbf,omitempty"`
	Subject    string   `json:"sub,omitempty"`
	Roles      []string `json:"roles,omitempty"`
	SessionTTL int64    `json:"sessionTTL,omitempty"`
}

func ValidateJwtToken(ServiceToken string, TenantUrl string) (*IdemeumClaims, error) {
	//Token Validation
	key, err := LoadPublicKey(TenantUrl + "/.well-known/jwks.json")
	if err != nil {
		log.Printf("Failed to load public key for tenant :%v \n", TenantUrl)
		return nil, trace.BadParameter("invalid configuration")
	}

	parser := &jwt.Parser{SkipClaimsValidation: true}
	token, err := parser.ParseWithClaims(ServiceToken, &IdemeumClaims{}, func(token *jwt.Token) (interface{}, error) {
		return key, nil
	})

	if err != nil {
		log.Print("Failed to validate the idemeum token", err)
		return nil, trace.BadParameter("Invalid idemeum token")
	}

	claims, ok := token.Claims.(*IdemeumClaims)
	if !ok && !token.Valid {
		log.Print("Failed to validate the idemeum token")
		return nil, trace.BadParameter("invalid token")
	}

	err = claims.Valid()
	if err != nil {
		log.Print("Failed to validate the idemeum token", err)
		return nil, trace.BadParameter("token invalid")
	}

	if !claims.VerifyIssuer(TenantUrl, true) {
		return nil, trace.BadParameter("token invalid Issuer")
	}

	if !claims.VerifyAudience(TenantUrl, true) {
		return nil, trace.BadParameter("token invalid audience")
	}
	return claims, nil
}

func doRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	client := http.DefaultClient
	if c, ok := ctx.Value(oauth2.HTTPClient).(*http.Client); ok {
		client = c
	}
	return client.Do(req.WithContext(ctx))
}

func LoadPublicKey(jwksURL string) (*ecdsa.PublicKey, error) {
	log.Printf("Load public keys using %v\n", jwksURL)
	req, err := http.NewRequest("GET", jwksURL, nil)
	if err != nil {
		return nil, fmt.Errorf("jwks: can't create request: %v", err)
	}

	resp, err := doRequest(context.TODO(), req)
	if err != nil {
		return nil, fmt.Errorf("failed to get  keys using jwks %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("jwks: unable to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jwks: get keys failed: %s %s", resp.Status, body)
	}

	var keySet jose.JSONWebKeySet
	err = json.Unmarshal(body, &keySet)
	if err != nil {
		return nil, fmt.Errorf("jwks: failed to decode keys: %v %s", err, body)
	}
	return keySet.Keys[0].Key.(*ecdsa.PublicKey), nil
}

func (c IdemeumClaims) Valid() error {
	now := TimeFunc().Unix()

	// The claims below are optional, by default, so if they are set to the
	// default value in Go, let's not fail the verification for them.
	if c.VerifyExpiresAt(now, false) == false {
		delta := time.Unix(now, 0).Sub(time.Unix(c.ExpiresAt, 0))
		return fmt.Errorf("token is expired by %v", delta)
	}

	if c.VerifyIssuedAt(now, false) == false {
		return fmt.Errorf("token used before issued")
	}

	if c.VerifyNotBefore(now, false) == false {
		return fmt.Errorf("token is not valid yet")
	}

	if c.Subject == "" {
		return fmt.Errorf("token missing subject claim")
	}

	return nil
}

func (c *IdemeumClaims) VerifyExpiresAt(cmp int64, req bool) bool {
	return verifyExp(c.ExpiresAt, cmp, req)
}

func (c *IdemeumClaims) VerifyIssuedAt(cmp int64, req bool) bool {
	return verifyIat(c.IssuedAt, cmp, req)
}

func (c *IdemeumClaims) VerifyIssuer(cmp string, req bool) bool {
	return verifyIss(c.Issuer, cmp, req)
}

func (c *IdemeumClaims) VerifyAudience(cmp string, req bool) bool {
	return verifyAud(c.Issuer, cmp, req)
}

func (c *IdemeumClaims) VerifyNotBefore(cmp int64, req bool) bool {
	return verifyNbf(c.NotBefore, cmp, req)
}

// ----- helpers

func verifyExp(exp int64, now int64, required bool) bool {
	if exp == 0 {
		return !required
	}
	return now <= exp
}

func verifyIat(iat int64, now int64, required bool) bool {
	if iat == 0 {
		return !required
	}
	return now >= iat
}

func verifyIss(iss string, cmp string, required bool) bool {
	if iss == "" {
		return !required
	}
	if subtle.ConstantTimeCompare([]byte(iss), []byte(cmp)) != 0 {
		return true
	} else {
		return false
	}
}

func verifyAud(aud string, cmp string, required bool) bool {
	if aud == "" {
		return !required
	}
	if subtle.ConstantTimeCompare([]byte(aud), []byte(cmp)) != 0 {
		return true
	} else {
		return false
	}
}

func verifyNbf(nbf int64, now int64, required bool) bool {
	if nbf == 0 {
		return !required
	}
	return now >= nbf
}
