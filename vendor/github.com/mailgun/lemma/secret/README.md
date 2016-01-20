secret
======

Mailgun tools for authenticated encryption.

**Overview**

Package secret provides tools for encrypting and decrypting authenticated messages.
Like all lemma packages, metrics are built in and can be emitted to check
for anomalous behavior.

[NaCl](http://nacl.cr.yp.to/) is the underlying secret-key authenticated encryption
library used. NaCl uses Salsa20 and Poly1305 as its cipher and MAC respectively.

**Examples**

_Key generation and use_

```go
package main

import (
    "github.com/mailgun/lemma/secret"
)

// generate a new randomly generated key. use this to create a new key.
keyBytes, err := secret.NewKey()

// read base64 encoded key in from disk
secretService, err := secret.New(&secret.Config{KeyPath: "/path/to/secret.key"})

// set key bytes directly
secretService, err := secret.New(&secret.Config{
    KeyBytes: &[32]byte{
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
    },
})

// given a base64 encoded key, return key bytes
secret.EncodedStringToKey("c3VycHJpc2UsIHRoaXMgaXMgYSBmYWtlIGtleSE=")

// given key bytes, return an base64 encoded key
secret.KeyToEncodedString(&[32]byte{
    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
})
```

---

_Encrypt message with existing key_


```go
import (
    "fmt"
    "encoding/base64"

    "github.com/mailgun/lemma/secret"
)

// create a new secret encryption service using the above generated key
s, err := secret.New(&secret.Config{KeyPath: "/path/to/secret.key"})
if err != nil {
    fmt.Printf("Got unexpected response from NewWithKeyBytes: %v\n", err)
}

// seal message
message := []byte("hello, world")
sealed, err := s.Seal(message)
if err != nil {
    fmt.Printf("Got unexpected response from Seal: %v\n", err)
}

// optionally base64 encode them and store them somewhere (like in a database)
ciphertext := base64.StdEncoding.EncodeToString(sealed.Ciphertext)
nonce := base64.StdEncoding.EncodeToString(sealed.Nonce)
fmt.Printf("Ciphertext: %v, Nonce: %v\n", ciphertext, nonce)
```

---

_Encrypt message with passed in key_

```go
import (
    "fmt"
    "github.com/mailgun/lemma/secret"
)

// create a new secret encryption service using the above generated key
s, err := secret.New(&secret.Config{KeyPath: "/path/to/secret.key"})
if err != nil {
    fmt.Printf("Got unexpected response from NewWithKeyBytes: %v\n", err)
}

// seal message
message := []byte("hello, world")
messageKey := secret.KeyBytes: &[32]byte{
    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
}
sealed, err := s.SealWithKey(message, messageKey)
if err != nil {
    fmt.Printf("Got unexpected response from Seal: %v\n", err)
}

fmt.Printf("Ciphertext: %v, Nonce: %v\n", sealed.Ciphertext, sealed.Nonce)
```

---

_Decrypt message_


```go
import (
    "fmt"
    "github.com/mailgun/lemma/secret"
)

// create a new secret encryption service using the above generated key
s, err := secret.New(&secret.Config{KeyPath: "/path/to/secret.key"})
if err != nil {
    fmt.Printf("Got unexpected response from NewWithKeyBytes: %v\n", err)
}

var ciphertext []byte
var nonce []byte

// read in ciphertext and nonce
[...]

// decrypt and open message
plaintext, err := s.Open(&secret.SealedBytes{
    Ciphertext: ciphertext,
    Nonce:      nonce,
})
if err != nil {
    fmt.Printf("Got unexpected response from Open: %v\n", err)
}

fmt.Printf("Plaintext: %v\n", plaintext)
```

---

_Emit Metrics_

```go
import (
    "fmt"
    "github.com/mailgun/lemma/secret"
)

// define statsd server for metrics
s, err := secret.New(&secret.Config{
    KeyPath:      "/path/to/secret.key",

    EmitStats:    true,
    StatsdHost:   "www.example.com",
    StatsdPort:   8125,
    StatsdPrefix: "a_secret_prefix",
})

// now, when using the service, success and failures will be emitted to statsd
plaintext, err := s.Open(...)
if err != nil {
    fmt.Printf("Got unexpected response from Open: %v\n", err)
}
```
