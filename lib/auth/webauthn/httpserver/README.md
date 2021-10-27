# Webauthn httpserver

The httpserver package contains a toy Webauthn HTTPS server used to test web
integration and the interplay between web-registered and tsh-registered devices.

Devices registered via either interface should be able to be used
interchangeably (with the exception of Touch ID, which is always tied to a
specific app).

This is meant for local testing and debugging only. Keep it far away from any
production uses.

## Why this exists?

* It is a simple way to test web integration
* It is a simple way to interact with Touch ID
* Device registration (currently) relies on a streaming RPC, which is difficult
  to interact with using pure Js

Note that browser Webauthn APIs need a secure context (or you have to disable a
bunch of checks in your browser, YMMV).

## Usage

1. Start the Teleport service
2. Start the httpserver

```shell
# Generate TLS certificates for the server
# (Eg, `cd; mkcert localhost 127.0.0.1 ::1`)

# Start the test server
# cd /path/to/teleport/repo
go run ./lib/auth/webauthn/httpserver/ \
  -cert_file ~/localhost+2.pem \
  -key_file ~/localhost+3-key.pem
```

3. Navigate to https://localhost:8080/index.html
4. ?
5. Profit
