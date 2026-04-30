package common

import (
	_ "unsafe"
)

//go:linkname http2disableExtendedConnectProtocol net/http/internal/http2.disableExtendedConnectProtocol
var http2disableExtendedConnectProtocol bool

func init() {
	// Enable WebSocket over HTTP/2 (RFC 8441) for app and database access features.
	//
	// Go's net/http package disables the extended CONNECT method (RFC 8441) by default
	// unless GODEBUG=http2xconnect=1 is set. However, that env var is read during the
	// net/http package's own init(), which runs before any init() in this package which means
	// os.Setenv cannot influence it from here.
	//
	// Instead, we use go:linkname to reach into the internal http2 package and directly
	// set the disableExtendedConnectProtocol flag to false, bypassing the GODEBUG check
	// entirely. This unconditionally enables the HTTP/2 extended CONNECT handler, which
	// is required for WebSocket proxying over HTTP/2 connections.
	//
	// Revert this change if the Go team adds an official API for enabling the extended CONNECT handler
	// - https://github.com/golang/go/issues/53208
	// - https://github.com/golang/go/issues/72071
	http2disableExtendedConnectProtocol = false
}
