// Copyright 2022 Gravitational, Inc
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

package tdp

import "image/png"

// PNGEncoder returns the encoder used for PNG Frames.
// It is not safe for concurrent use.
func PNGEncoder() *png.Encoder {
	return &png.Encoder{
		CompressionLevel: png.BestSpeed,
		BufferPool:       &pool{},
	}
}

// pool implements png.EncoderBufferPool,
// allowing us to reuse encoding resources
type pool struct {
	b *png.EncoderBuffer
}

// all encoding happens in a single thread, so we don't
// need anything as sophisticated as a sync.Pool here

func (p *pool) Get() *png.EncoderBuffer   { return p.b }
func (p *pool) Put(eb *png.EncoderBuffer) { p.b = eb }
