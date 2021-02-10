package server

import (
	"crypto/tls"
	"fmt"
	"sync"

	. "github.com/siddontang/go-mysql/mysql"
)

var defaultServer = NewDefaultServer()

// Defines a basic MySQL server with configs.
//
// We do not aim at implementing the whole MySQL connection suite to have the best compatibilities for the clients.
// The MySQL server can be configured to switch auth methods covering 'mysql_old_password', 'mysql_native_password',
// 'mysql_clear_password', 'authentication_windows_client', 'sha256_password', 'caching_sha2_password', etc.
//
// However, since some old auth methods are considered broken with security issues. MySQL major versions like 5.7 and 8.0 default to
// 'mysql_native_password' or 'caching_sha2_password', and most MySQL clients should have already supported at least one of the three auth
// methods 'mysql_native_password', 'caching_sha2_password', and 'sha256_password'. Thus here we will only support these three
// auth methods, and use 'mysql_native_password' as default for maximum compatibility with the clients and leave the other two as
// config options.
//
// The MySQL doc states that 'mysql_old_password' will be used if 'CLIENT_PROTOCOL_41' or 'CLIENT_SECURE_CONNECTION' flag is not set.
// We choose to drop the support for insecure 'mysql_old_password' auth method and require client capability 'CLIENT_PROTOCOL_41' and 'CLIENT_SECURE_CONNECTION'
// are set. Besides, if 'CLIENT_PLUGIN_AUTH' is not set, we fallback to 'mysql_native_password' auth method.
type Server struct {
	serverVersion     string // e.g. "8.0.12"
	protocolVersion   int    // minimal 10
	capability        uint32 // server capability flag
	collationId       uint8
	defaultAuthMethod string // default authentication method, 'mysql_native_password'
	pubKey            []byte
	tlsConfig         *tls.Config
	cacheShaPassword  *sync.Map // 'user@host' -> SHA256(SHA256(PASSWORD))
}

// NewDefaultServer: New mysql server with default settings.
//
// NOTES:
// TLS support will be enabled by default with auto-generated CA and server certificates (however, you can still use
// non-TLS connection). By default, it will verify the client certificate if present. You can enable TLS support on
// the client side without providing a client-side certificate. So only when you need the server to verify client
// identity for maximum security, you need to set a signed certificate for the client.
func NewDefaultServer() *Server {
	caPem, caKey := generateCA()
	certPem, keyPem := generateAndSignRSACerts(caPem, caKey)
	tlsConf := NewServerTLSConfig(caPem, certPem, keyPem, tls.VerifyClientCertIfGiven)
	return &Server{
		serverVersion:   "5.7.0",
		protocolVersion: 10,
		capability: CLIENT_LONG_PASSWORD | CLIENT_LONG_FLAG | CLIENT_CONNECT_WITH_DB | CLIENT_PROTOCOL_41 |
			CLIENT_TRANSACTIONS | CLIENT_SECURE_CONNECTION | CLIENT_PLUGIN_AUTH | CLIENT_SSL | CLIENT_PLUGIN_AUTH_LENENC_CLIENT_DATA,
		collationId:       DEFAULT_COLLATION_ID,
		defaultAuthMethod: AUTH_NATIVE_PASSWORD,
		pubKey:            getPublicKeyFromCert(certPem),
		tlsConfig:         tlsConf,
		cacheShaPassword:  new(sync.Map),
	}
}

// NewServer: New mysql server with customized settings.
//
// NOTES:
// You can control the authentication methods and TLS settings here.
// For auth method, you can specify one of the supported methods 'mysql_native_password', 'caching_sha2_password', and 'sha256_password'.
// The specified auth method will be enforced by the server in the connection phase. That means, client will be asked to switch auth method
// if the supplied auth method is different from the server default.
// And for TLS support, you can specify self-signed or CA-signed certificates and decide whether the client needs to provide
// a signed or unsigned certificate to provide different level of security.
func NewServer(serverVersion string, collationId uint8, defaultAuthMethod string, pubKey []byte, tlsConfig *tls.Config) *Server {
	if !isAuthMethodSupported(defaultAuthMethod) {
		panic(fmt.Sprintf("server authentication method '%s' is not supported", defaultAuthMethod))
	}

	//if !isAuthMethodAllowedByServer(defaultAuthMethod, allowedAuthMethods) {
	//	panic(fmt.Sprintf("default auth method is not one of the allowed auth methods"))
	//}
	var capFlag = CLIENT_LONG_PASSWORD | CLIENT_LONG_FLAG | CLIENT_CONNECT_WITH_DB | CLIENT_PROTOCOL_41 |
		CLIENT_TRANSACTIONS | CLIENT_SECURE_CONNECTION | CLIENT_PLUGIN_AUTH | CLIENT_PLUGIN_AUTH_LENENC_CLIENT_DATA
	if tlsConfig != nil {
		capFlag |= CLIENT_SSL
	}
	return &Server{
		serverVersion:     serverVersion,
		protocolVersion:   10,
		capability:        capFlag,
		collationId:       collationId,
		defaultAuthMethod: defaultAuthMethod,
		pubKey:            pubKey,
		tlsConfig:         tlsConfig,
		cacheShaPassword:  new(sync.Map),
	}
}

func isAuthMethodSupported(authMethod string) bool {
	return authMethod == AUTH_NATIVE_PASSWORD || authMethod == AUTH_CACHING_SHA2_PASSWORD || authMethod == AUTH_SHA256_PASSWORD
}

func (s *Server) InvalidateCache(username string, host string) {
	s.cacheShaPassword.Delete(fmt.Sprintf("%s@%s", username, host))
}
