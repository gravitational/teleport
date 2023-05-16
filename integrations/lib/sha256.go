/*
Copyright 2021 Gravitational, Inc.

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

package lib

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"io"
	"os"

	"github.com/gravitational/trace"
)

type SHA256Sum [sha256.Size]byte

type SHA256 struct {
	hash hash.Hash
}

func NewSHA256() SHA256 {
	hash := sha256.New()
	return SHA256{hash: hash}
}

func (s SHA256) Write(p []byte) (n int, err error) {
	return s.hash.Write(p)
}

func (s SHA256) Sum() SHA256Sum {
	var result SHA256Sum
	copy(result[:], s.hash.Sum(nil)[:sha256.Size])
	return result
}

func ReadFileSHA256(fileName string) (SHA256Sum, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return SHA256Sum{}, trace.Wrap(err)
	}
	sha256 := NewSHA256()
	_, err = io.Copy(sha256, file)
	if err = trace.NewAggregate(err, file.Close()); err != nil {
		return SHA256Sum{}, trace.Wrap(err)
	}
	return sha256.Sum(), nil
}

func MustHexSHA256(str string) SHA256Sum {
	data, err := hex.DecodeString(str)
	if err != nil {
		panic(err)
	}
	var result SHA256Sum
	copy(result[:], data[:sha256.Size])
	return result
}
