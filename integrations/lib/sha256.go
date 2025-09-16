/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
