/*

 Copyright 2022 Gravitational, Inc.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.


*/

package cassandra

import "io"

type memoryBuffer struct {
	buff []byte
	r    io.Reader
}

func newMemoryBuffer(reader io.Reader) *memoryBuffer {
	return &memoryBuffer{
		buff: make([]byte, 0, 4096),
		r:    reader,
	}
}

func (b *memoryBuffer) Read(p []byte) (int, error) {
	n, err := b.r.Read(p)
	b.buff = append(b.buff, p[:n]...)
	return n, err
}

func (b *memoryBuffer) Bytes() []byte {
	return b.buff
}

func (b *memoryBuffer) Reset() {
	b.buff = b.buff[:0]
}
