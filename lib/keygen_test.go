package lib

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/gravitational/teleport/api/constants"
)

func BenchmarkRSAKeygen1(b *testing.B) {
	var err error

	for i := 0; i < b.N; i++ {
		_, err = rsa.GenerateKey(rand.Reader, constants.RSAKeySize)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRSAKeygen10(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for j := 0; j < 10; j++ {
			_, err := rsa.GenerateKey(rand.Reader, constants.RSAKeySize)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}

func BenchmarkRSAKeygen100(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for j := 0; j < 100; j++ {
			_, err := rsa.GenerateKey(rand.Reader, constants.RSAKeySize)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}

func BenchmarkRSAKeygen1000(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			_, err := rsa.GenerateKey(rand.Reader, constants.RSAKeySize)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}
