package server

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/tls"
	"fmt"

	"github.com/pingcap/errors"
	. "github.com/siddontang/go-mysql/mysql"
)

var ErrAccessDenied = errors.New("access denied")

func (c *Conn) compareAuthData(authPluginName string, clientAuthData []byte) error {
	switch authPluginName {
	case AUTH_NATIVE_PASSWORD:
		if err := c.acquirePassword(); err != nil {
			return err
		}
		return c.compareNativePasswordAuthData(clientAuthData, c.password)

	case AUTH_CACHING_SHA2_PASSWORD:
		if err := c.compareCacheSha2PasswordAuthData(clientAuthData); err != nil {
			return err
		}
		if c.cachingSha2FullAuth {
			return c.handleAuthSwitchResponse()
		}
		return nil

	case AUTH_SHA256_PASSWORD:
		if err := c.acquirePassword(); err != nil {
			return err
		}
		cont, err := c.handlePublicKeyRetrieval(clientAuthData)
		if err != nil {
			return err
		}
		if !cont {
			return nil
		}
		return c.compareSha256PasswordAuthData(clientAuthData, c.password)

	default:
		return errors.Errorf("unknown authentication plugin name '%s'", authPluginName)
	}
}

func (c *Conn) acquirePassword() error {
	password, found, err := c.credentialProvider.GetCredential(c.user)
	if err != nil {
		return err
	}
	if !found {
		return NewDefaultError(ER_NO_SUCH_USER, c.user, c.RemoteAddr().String())
	}
	c.password = password
	return nil
}

func scrambleValidation(cached, nonce, scramble []byte) bool {
	// SHA256(SHA256(SHA256(STORED_PASSWORD)), NONCE)
	crypt := sha256.New()
	crypt.Write(cached)
	crypt.Write(nonce)
	message2 := crypt.Sum(nil)
	// SHA256(PASSWORD)
	if len(message2) != len(scramble) {
		return false
	}
	for i := range message2 {
		message2[i] ^= scramble[i]
	}
	// SHA256(SHA256(PASSWORD)
	crypt.Reset()
	crypt.Write(message2)
	m := crypt.Sum(nil)
	return bytes.Equal(m, cached)
}

func (c *Conn) compareNativePasswordAuthData(clientAuthData []byte, password string) error {
	if bytes.Equal(CalcPassword(c.salt, []byte(c.password)), clientAuthData) {
		return nil
	}
	return ErrAccessDenied
}

func (c *Conn) compareSha256PasswordAuthData(clientAuthData []byte, password string) error {
	// Empty passwords are not hashed, but sent as empty string
	if len(clientAuthData) == 0 {
		if password == "" {
			return nil
		}
		return ErrAccessDenied
	}
	if tlsConn, ok := c.Conn.Conn.(*tls.Conn); ok {
		if !tlsConn.ConnectionState().HandshakeComplete {
			return errors.New("incomplete TSL handshake")
		}
		// connection is SSL/TLS, client should send plain password
		// deal with the trailing \NUL added for plain text password received
		if l := len(clientAuthData); l != 0 && clientAuthData[l-1] == 0x00 {
			clientAuthData = clientAuthData[:l-1]
		}
		if bytes.Equal(clientAuthData, []byte(password)) {
			return nil
		}
		return ErrAccessDenied
	} else {
		// client should send encrypted password
		// decrypt
		dbytes, err := rsa.DecryptOAEP(sha1.New(), rand.Reader, (c.serverConf.tlsConfig.Certificates[0].PrivateKey).(*rsa.PrivateKey), clientAuthData, nil)
		if err != nil {
			return err
		}
		plain := make([]byte, len(password)+1)
		copy(plain, password)
		for i := range plain {
			j := i % len(c.salt)
			plain[i] ^= c.salt[j]
		}
		if bytes.Equal(plain, dbytes) {
			return nil
		}
		return ErrAccessDenied
	}
}

func (c *Conn) compareCacheSha2PasswordAuthData(clientAuthData []byte) error {
	// Empty passwords are not hashed, but sent as empty string
	if len(clientAuthData) == 0 {
		if err := c.acquirePassword(); err != nil {
			return err
		}
		if c.password == "" {
			return nil
		}
		return ErrAccessDenied
	}
	// the caching of 'caching_sha2_password' in MySQL, see: https://dev.mysql.com/worklog/task/?id=9591
	if _, ok := c.credentialProvider.(*InMemoryProvider); ok {
		// since we have already kept the password in memory and calculate the scramble is not that high of cost, we eliminate
		// the caching part. So our server will never ask the client to do a full authentication via RSA key exchange and it appears
		// like the auth will always hit the cache.
		if err := c.acquirePassword(); err != nil {
			return err
		}
		if bytes.Equal(CalcCachingSha2Password(c.salt, c.password), clientAuthData) {
			// 'fast' auth: write "More data" packet (first byte == 0x01) with the second byte = 0x03
			return c.writeAuthMoreDataFastAuth()
		}
		return ErrAccessDenied
	}
	// other type of credential provider, we use the cache
	cached, ok := c.serverConf.cacheShaPassword.Load(fmt.Sprintf("%s@%s", c.user, c.Conn.LocalAddr()))
	if ok {
		// Scramble validation
		if scrambleValidation(cached.([]byte), c.salt, clientAuthData) {
			// 'fast' auth: write "More data" packet (first byte == 0x01) with the second byte = 0x03
			return c.writeAuthMoreDataFastAuth()
		}
		return ErrAccessDenied
	}
	// cache miss, do full auth
	if err := c.writeAuthMoreDataFullAuth(); err != nil {
		return err
	}
	c.cachingSha2FullAuth = true
	return nil
}
