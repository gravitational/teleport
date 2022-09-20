random
======

Interface for random number generators.

**Overview**

Provides an interface for all random number generators used by `lemma`. Allows
random number generation to be done in one place as well as faking random
numbers when predictable output is required (like in tests).

The cryptographically secure pseudo-random number generator (CSPRNG) used is `/dev/urandom`.

**Examples**

_Generate a n-bit random number_

```go
import (
    "fmt"

    "github.com/mailgun/lemma/random"
)

outputSize = 16 // 128 bit = 16 byte

csprng := random.CSPRNG{}

// hex-encoded random bytes
randomHexDigest, err := csprng.HexDigest(outputSize)
fmt.Printf("Random Hex Digest: %v, err: %v\n", randomHexDigest, err)

// raw bytes
randomBytes, err := csprng.Bytes(outputSize)
fmt.Printf("Random Bytes: %# x, err: %v\n", randomBytes, err)
```

Will print out something like the following to the console:

```
Random Hex Digest: 45eb93fdf5afe149ee3e61412c97e9bc, err: <nil>
Random Bytes: 0xee 0x52 0x5b 0x50 0xb9 0x10 0x3c 0x14 0x75 0x9a 0xa5 0xb9 0xa3 0xc4 0x6e 0x50, err: <nil>
```

_Generate a fake n-bit random number_

```go
import (
    "fmt"

    "github.com/mailgun/lemma/random"
)

outputSize = 16 // 128 bit = 16 byte

fakerng := random.FakeRNG{}

// (fake) hex-encoded random bytes
fakeRandomHexDigest := fakerng.HexDigest(outputSize)
fmt.Printf("(Fake) Random Hex Digest: %v\n", fakeHexDigest)

// (fake) raw bytes
fakeRandomBytes := fakerng.Bytes(outputSize)
fmt.Printf("(Fake) Random Bytes: %# x\n", fakeRandomBytes)
```

Will print out the following to the console:

```
(Fake) Random Hex Digest: 0102030405060708090A0B0C0E0F1011
(Fake) Random Bytes: 0x00 0x01 0x02 0x03 0x04 0x05 0x06 0x07 0x08 0x09 0x0a 0x0b 0x0c 0x0d 0x0e 0x0f
```
