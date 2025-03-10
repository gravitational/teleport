package cryptopatch

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"fmt"
	"io"
	"os"
	"path"
	"sync"
)

var (
	f = sync.OnceValue(func() *os.File {
		f, err := os.OpenFile(path.Join(os.TempDir(), "cryptolog.csv"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}
		return f
	})
)

func log(alg string) {
	fmt.Fprintln(f(), alg, ",", 1)
}

func GenerateRSAKey(random io.Reader, bits int) (*rsa.PrivateKey, error) {
	log("RSA")
	return rsa.GenerateKey(random, bits)
}

func GenerateECDSAKey(c elliptic.Curve, rand io.Reader) (*ecdsa.PrivateKey, error) {
	log("ECDSA")
	return ecdsa.GenerateKey(c, rand)
}

func GenerateEd25519Key(rand io.Reader) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	log("Ed25519")
	return ed25519.GenerateKey(rand)
}
