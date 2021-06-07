// Copyright 2019 Google LLC
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

package keyset

import (
	"io"
	"io/ioutil"

	"github.com/golang/protobuf/proto"

	tinkpb "github.com/google/tink/go/proto/tink_go_proto"
)

// BinaryReader deserializes a keyset from binary proto format.
type BinaryReader struct {
	r io.Reader
}

// NewBinaryReader returns new BinaryReader that will read from r.
func NewBinaryReader(r io.Reader) *BinaryReader {
	return &BinaryReader{r: r}
}

// Read parses a (cleartext) keyset from the underlying io.Reader.
func (bkr *BinaryReader) Read() (*tinkpb.Keyset, error) {
	keyset := &tinkpb.Keyset{}

	if err := read(bkr.r, keyset); err != nil {
		return nil, err
	}
	return keyset, nil
}

// ReadEncrypted parses an EncryptedKeyset from the underlying io.Reader.
func (bkr *BinaryReader) ReadEncrypted() (*tinkpb.EncryptedKeyset, error) {
	keyset := &tinkpb.EncryptedKeyset{}

	if err := read(bkr.r, keyset); err != nil {
		return nil, err
	}
	return keyset, nil
}

func read(r io.Reader, msg proto.Message) error {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	return proto.Unmarshal(data, msg)
}

// BinaryWriter serializes a keyset into binary proto format.
type BinaryWriter struct {
	w io.Writer
}

// NewBinaryWriter returns a new BinaryWriter that will write to w.
func NewBinaryWriter(w io.Writer) *BinaryWriter {
	return &BinaryWriter{w: w}
}

// Write writes the keyset to the underlying io.Writer.
func (bkw *BinaryWriter) Write(keyset *tinkpb.Keyset) error {
	return write(bkw.w, keyset)
}

// WriteEncrypted writes the encrypted keyset to the underlying io.Writer.
func (bkw *BinaryWriter) WriteEncrypted(keyset *tinkpb.EncryptedKeyset) error {
	return write(bkw.w, keyset)
}

func write(w io.Writer, msg proto.Message) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		return err
	}

	_, err = w.Write(data)
	return err
}
