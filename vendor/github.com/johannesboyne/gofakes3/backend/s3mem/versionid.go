package s3mem

import (
	"encoding/base32"
	"fmt"
	"math/big"
	"sync"

	"github.com/johannesboyne/gofakes3"
)

var add1 = new(big.Int).SetInt64(1)

type versionGenerator struct {
	state uint64
	size  int
	next  *big.Int
	mu    sync.Mutex
}

func newVersionGenerator(seed uint64, size int) *versionGenerator {
	if size <= 0 {
		size = 64
	}
	return &versionGenerator{next: new(big.Int), state: seed}
}

func (v *versionGenerator) Next(scratch []byte) (gofakes3.VersionID, []byte) {
	v.mu.Lock()

	v.next.Add(v.next, add1)
	idb := []byte(fmt.Sprintf("%030d", v.next))

	neat := v.size/8*8 + 8 // cheap and nasty way to ensure a multiple of 8 definitely greater than size

	scratchLen := len(idb) + neat + 1
	if len(scratch) < scratchLen {
		scratch = make([]byte, scratchLen)
	}
	copy(scratch, idb)

	b := scratch[len(idb)+1:]

	// This is a simple inline implementation of http://xoshiro.di.unimi.it/splitmix64.c.
	// It may not ultimately be the right tool for this job but with a large
	// enough size the collision risk should still be minuscule.
	for i := 0; i < neat; i += 8 {
		v.state += 0x9E3779B97F4A7C15
		z := v.state
		z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
		z = (z ^ (z >> 27)) * 0x94D049BB133111EB
		b[i], b[i+1], b[i+2], b[i+3], b[i+4], b[i+5], b[i+6], b[i+7] =
			byte(z), byte(z>>8), byte(z>>16), byte(z>>24), byte(z>>32), byte(z>>40), byte(z>>48), byte(z>>56)
	}

	v.mu.Unlock()

	// The version IDs that come out of S3 appear to start with '3/' and follow
	// with a base64-URL encoded blast of god knows what. There didn't appear
	// to be any explanation of the format beyond that, but let's copy it anyway.
	//
	// Base64 is not sortable though, and we need our versions to be lexicographically
	// sortable for the SkipList key, so we have to encode it as base32hex, which _is_
	// sortable, and just pretend that it's "Base64". Phew!

	return gofakes3.VersionID(fmt.Sprintf("3/%s", base32.HexEncoding.EncodeToString(scratch))), scratch
}
