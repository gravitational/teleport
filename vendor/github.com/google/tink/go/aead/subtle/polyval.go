// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
////////////////////////////////////////////////////////////////////////////////

package subtle

import (
	"encoding/binary"
	"fmt"
)

const (
	// PolyvalBlockSize is the block size (in bytes) that POLYVAL uses.
	PolyvalBlockSize = 16

	u32Sel0 uint32 = 0x11111111
	u32Sel1 uint32 = 0x22222222
	u32Sel2 uint32 = 0x44444444
	u32Sel3 uint32 = 0x88888888

	u64Sel0 uint64 = 0x1111111111111111
	u64Sel1 uint64 = 0x2222222222222222
	u64Sel2 uint64 = 0x4444444444444444
	u64Sel3 uint64 = 0x8888888888888888
)

// Polyval (RFC 8452) is a universal hash function which operates on GF(2^128)
// and can be used for constructing a Message Authentication Code (MAC).
// See Section 3 of go/rfc/8452 for definition.
type Polyval interface {
	// update the accumulator in the object with the blocks from data. If data
	// is not a multiple of 16 bytes, it is automatically zero padded.
	Update(data []byte)

	// finish completes the polyval computation and returns the result.
	Finish() [PolyvalBlockSize]byte
}

// fieldElement represents a value in GF(2^128).
// In order to reflect the Polyval standard and make binary.LittleEndian suitable
// for marshaling these values, the bits are stored in little endian order.
// For example:
//   the coefficient of x^0 can be obtained by v.lo & 1.
//   the coefficient of x^63 can be obtained by v.lo >> 63.
//   the coefficient of x^64 can be obtained by v.hi & 1.
//   the coefficient of x^127 can be obtained by v.hi >> 63.
type fieldElement struct {
	lo, hi uint64
}

// polyval implements the POLYVAL function as defined by go/rfc/8452.
type polyval struct {
	key fieldElement
	acc fieldElement
}

// Assert that polyval implements Polyval interface
var _ Polyval = (*polyval)(nil)

// mul32 multiplies two 32 bit polynomials in GF(2^128) using Karatsuba multiplication.
func mul32(a uint32, b uint32) uint64 {
	a0 := uint64(a & u32Sel0)
	a1 := uint64(a & u32Sel1)
	a2 := uint64(a & u32Sel2)
	a3 := uint64(a & u32Sel3)

	b0 := uint64(b & u32Sel0)
	b1 := uint64(b & u32Sel1)
	b2 := uint64(b & u32Sel2)
	b3 := uint64(b & u32Sel3)

	c0 := (a0 * b0) ^ (a1 * b3) ^ (a2 * b2) ^ (a3 * b1)
	c1 := (a0 * b1) ^ (a1 * b0) ^ (a2 * b3) ^ (a3 * b2)
	c2 := (a0 * b2) ^ (a1 * b1) ^ (a2 * b0) ^ (a3 * b3)
	c3 := (a0 * b3) ^ (a1 * b2) ^ (a2 * b1) ^ (a3 * b0)

	return (c0 & u64Sel0) | (c1 & u64Sel1) | (c2 & u64Sel2) | (c3 & u64Sel3)
}

// mul64 multiplies two 64 bit polynomials in GF(2^128) using Karatsuba multiplication.
func mul64(a uint64, b uint64) fieldElement {
	a0 := uint32(a & 0xffffffff)
	a1 := uint32(a >> 32)

	b0 := uint32(b & 0xffffffff)
	b1 := uint32(b >> 32)

	lo := mul32(a0, b0)
	hi := mul32(a1, b1)
	mid := mul32(a0^a1, b0^b1) ^ lo ^ hi

	return fieldElement{lo: lo ^ (mid << 32), hi: hi ^ (mid >> 32)}
}

// polyvalDot implements the dot operation defined by go/rfc/8452.
// dot(a, b) = a * b * x^-128.
// The value of the field element x^-128 is equal to x^127 + x^124 + x^121 + x^114 + 1.
// The result of this multiplication, dot(a, b), is another field element.
// The implementation here is inspired from BoringSSL's implementation of gcm_polyval_nohw().
// Ref: https://boringssl.googlesource.com/boringssl/+/master/crypto/fipsmodule/modes/gcm_nohw.c
func polyvalDot(a fieldElement, b fieldElement) fieldElement {
	// Karatsuba multiplication. The product of |a| and |b| is stored in |r0| and |r1|
	// Note there is no byte or bit reversal because we are evaluating POLYVAL.
	r0 := mul64(a.lo, b.lo)
	r1 := mul64(a.hi, b.hi)

	mid := mul64(a.lo^a.hi, b.lo^b.hi)
	mid.lo ^= r0.lo ^ r1.lo
	mid.hi ^= r0.hi ^ r1.hi

	r1.lo ^= mid.hi
	r0.hi ^= mid.lo

	// Now we multiply our 256-bit result by x^-128 and reduce.
	// |r1| shifts into position and we must multiply |r0| by x^-128. We have:
	//
	//       1 = x^121 + x^126 + x^127 + x^128
	//  x^-128 = x^-7 + x^-2 + x^-1 + 1
	//
	// This is the GHASH reduction step, but with bits flowing in reverse.
	// The x^-7, x^-2, and x^-1 terms shift bits past x^0, which would require
	// another reduction steps. Instead, we gather the excess bits, incorporate
	// them into |r0| and reduce once.
	// Ref: slides 17-19 of https://crypto.stanford.edu/RealWorldCrypto/slides/gueron.pdf.
	r0.hi ^= (r0.lo << 63) ^ (r0.lo << 62) ^ (r0.lo << 57)

	// 1
	r1.lo ^= r0.lo
	r1.hi ^= r0.hi

	// x^-1
	r1.lo ^= r0.lo >> 1
	r1.lo ^= r0.hi << 63
	r1.hi ^= r0.hi >> 1

	// x^-2
	r1.lo ^= r0.lo >> 2
	r1.lo ^= r0.hi << 62
	r1.hi ^= r0.hi >> 2

	// x^-7
	r1.lo ^= r0.lo >> 7
	r1.lo ^= r0.hi << 57
	r1.hi ^= r0.hi >> 7

	return r1
}

// NewPolyval returns a Polyval instance.
func NewPolyval(key []byte) (Polyval, error) {
	if len(key) != PolyvalBlockSize {
		return nil, fmt.Errorf("polyval: Invalid key size: %d", len(key))
	}

	return &polyval{
		key: fieldElement{
			lo: binary.LittleEndian.Uint64(key[:8]),
			hi: binary.LittleEndian.Uint64(key[8:]),
		},
	}, nil
}

func (p *polyval) Update(data []byte) {
	var block []byte
	for len(data) > 0 {
		if len(data) >= PolyvalBlockSize {
			block = data[:PolyvalBlockSize]
			data = data[PolyvalBlockSize:]
		} else {
			var partialBlock [PolyvalBlockSize]byte
			copy(partialBlock[:], data)
			block = partialBlock[:]
			data = data[len(data):]
		}

		p.acc.lo ^= binary.LittleEndian.Uint64(block[:8])
		p.acc.hi ^= binary.LittleEndian.Uint64(block[8:])
		p.acc = polyvalDot(p.acc, p.key)
	}
}

func (p *polyval) Finish() (hash [PolyvalBlockSize]byte) {
	binary.LittleEndian.PutUint64(hash[:8], p.acc.lo)
	binary.LittleEndian.PutUint64(hash[8:], p.acc.hi)
	return
}
