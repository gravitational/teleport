# Go FIDO U2F Library

This Go package implements the parts of the FIDO U2F specification required on
the server side of an application.

## Features

- Native Go implementation
- No dependancies other than the Go standard library
- Token attestation certificate verification

## Usage

Please visit http://godoc.org/github.com/tstranex/u2f for the full
documentation.

### How to enrol a new token

```go
app_id := "http://localhost"

// Send registration request to the browser.
c, _ := NewChallenge(app_id, []string{app_id})
req, _ := c.RegisterRequest()

// Read response from the browser.
var resp RegisterResponse
reg, err := Register(resp, c, nil)
if err != nil {
    // Registration failed.
}

// Store registration in the database.
```

### How to perform an authentication

```go
// Fetch registration and counter from the database.
var reg Registration
var counter uint32

// Send authentication request to the browser.
c, _ := NewChallenge(app_id, []string{app_id})
req, _ := c.SignRequest(reg)

// Read response from the browser.
var resp SignResponse
newCounter, err := reg.Authenticate(resp, c, counter)
if err != nil {
    // Authentication failed.
}

// Store updated counter in the database.
```

## Installation

```
$ go get github.com/tstranex/u2f
```

## Example

See u2fdemo/main.go for an full example server. To run it:

```
$ go install github.com/tstranex/u2f/u2fdemo
$ ./bin/u2fdemo
```

Open https://localhost:3483 in Chrome.
Ignore the SSL warning (due to the self-signed certificate for localhost).
You can then test registering and authenticating using your token.

## License

The Go FIDO U2F Library is licensed under the MIT License.
