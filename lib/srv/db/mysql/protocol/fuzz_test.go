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

package protocol

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func unverifiedBase64Bytes(str string) []byte {
	bytes, _ := base64.StdEncoding.DecodeString(str)
	return bytes
}

func FuzzParsePacket(f *testing.F) {
	f.Add([]byte("00000"))
	f.Add(unverifiedBase64Bytes("FQAAAPoBAAAAgAD+AAgAAQDIAAAAAAAAAA=="))
	f.Add([]byte{0x1, 0x0, 0x0, 0x0, 0xd})
	f.Add([]byte{0xa, 0x0, 0x0, 0x0, 0x18, 0x5, 0x0, 0x0, 0x0, 0x2, 0x0, 0x62, 0x6f, 0x62})
	f.Add([]byte{0x5, 0x0, 0x0, 0x0, 0xc, 0x15, 0x0, 0x0, 0x0})
	f.Add([]byte{0x2, 0x0, 0x0, 0x0, 0x7, 0x40})
	f.Add([]byte{0x5, 0x0, 0x0, 0x0, 0x19, 0x1, 0x0, 0x0, 0x0})
	f.Add([]byte{0x1, 0x0, 0x0, 0x0, 0x1})
	f.Add(unverifiedBase64Bytes("HgAAABcCAAAAAAEAAAAAAf4ACAAFaGVsbG/IAAAAAAAAAA=="))
	f.Add([]byte{0x9, 0x0, 0x0, 0x0, 0x16, 0x73, 0x65, 0x6c, 0x65, 0x63, 0x74, 0x20, 0x31})
	f.Add([]byte{0x5, 0x0, 0x0, 0x0, 0x1a, 0x1, 0x0, 0x0, 0x0})
	f.Add([]byte{0x9, 0x0, 0x0, 0x0, 0x1c, 0x1, 0x0, 0x0, 0x0, 0xa, 0x0, 0x0, 0x0})
	f.Add([]byte{0x4, 0x0, 0x0, 0x0, 0x11, 0x62, 0x6f, 0x62})
	f.Add([]byte{0x9, 0x0, 0x0, 0x0, 0x3, 0x73, 0x65, 0x6c, 0x65, 0x63, 0x74, 0x20, 0x31})
	f.Add([]byte{0x5, 0x0, 0x0, 0x0, 0x5, 0x74, 0x65, 0x73, 0x74})
	f.Add([]byte{0x1, 0x0, 0x0, 0x0, 0x44})
	f.Add([]byte{0x2, 0x0, 0x0, 0x0, 0x8, 0x0})
	f.Add([]byte{0x9, 0x0, 0x0, 0x0, 0xff, 0x51, 0x4, 0x64, 0x65, 0x6e, 0x69, 0x65, 0x64})
	f.Add([]byte{0x5, 0x0, 0x0, 0x4, 0x11, 0x62, 0x6f, 0x62, 0x0})
	f.Add(unverifiedBase64Bytes("DwAAAP9RBCNIWTAwMGRlbmllZA=="))
	f.Add([]byte{0x5, 0x0, 0x0, 0x0, 0x2, 0x74, 0x65, 0x73, 0x74})
	f.Add([]byte{0x3, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0})
	f.Add([]byte{0xff, 0xff, 0xff, 0x0, 0x1})
	f.Add([]byte{0x5, 0x0, 0x0, 0x0, 0x6, 0x74, 0x65, 0x73, 0x74})
	f.Add(unverifiedBase64Bytes("bgAAAf9pBEhvc3QgJzcwLjE0NC4xMjguNDQnIGlzIGJsb2NrZWQgYmVjYXVzZSBvZiBtYW55IGNv" +
		"bm5lY3Rpb24gZXJyb3JzOyB1bmJsb2NrIHdpdGggJ21hcmlhZGItYWRtaW4gZmx1c2gtaG9zdHMn"))
	f.Add(unverifiedBase64Bytes("KgIAAP9RBENvdWxkIG5vdCBjb25uZWN0IHRvIGRhdGFiYXNlOgoKICBFUlJPUiAxMDQ1ICgyODAw" +
		"MCk6IEFjY2VzcyBkZW5pZWQgZm9yIHVzZXIgJ3Jvb3QnQCcxMjcuMC4wLjE6NTMxMDQnICh1c2lu" +
		"ZyBwYXNzd29yZDogWUVTKQoKTWFrZSBzdXJlIHRoYXQgSUFNIGF1dGggaXMgZW5hYmxlZCBmb3Ig" +
		"TXlTUUwgdXNlciAicm9vdCIgYW5kIFRlbGVwb3J0IGRhdGFiYXNlCmFnZW50J3MgSUFNIHBvbGlj" +
		"eSBoYXMgInJkcy1jb25uZWN0IiBwZXJtaXNzaW9ucyAobm90ZSB0aGF0IElBTSBjaGFuZ2VzIG1h" +
		"eQp0YWtlIGEgZmV3IG1pbnV0ZXMgdG8gcHJvcGFnYXRlKToKCnsKICAgICJWZXJzaW9uIjogIjIw" +
		"MTItMTAtMTciLAogICAgIlN0YXRlbWVudCI6IFsKICAgICAgICB7CiAgICAgICAgICAgICJFZmZl" +
		"Y3QiOiAiQWxsb3ciLAogICAgICAgICAgICAiQWN0aW9uIjogInJkcy1kYjpjb25uZWN0IiwKICAg" +
		"ICAgICAgICAgIlJlc291cmNlIjogImFybjphd3M6cmRzLWRiOnVzLWVhc3QtMTp7YWNjb3VudF9p" +
		"ZH06ZGJ1c2VyOntyZXNvdXJjZV9pZH0vKiIKICAgICAgICB9CiAgICBdCn0K"))
	f.Add(unverifiedBase64Bytes("XAAAAP9RBEVSUk9SIDEwNDUgKDI4MDAwKTogQWNjZXNzIGRlbmllZCBmb3IgdXNlciAncm9vdCdA" +
		"JzEyNy4wLjAuMTo0MjY2OCcgKHVzaW5nIHBhc3N3b3JkOiBZRVMp"))
	f.Add(unverifiedBase64Bytes("LAAAAP9RBGV4Y2VlZGVkIGNvbm5lY3Rpb24gbGltaXQgZm9yICIxMjcuMC4wLjEi"))
	f.Add(unverifiedBase64Bytes("VwAAAP9RBGFjY2VzcyB0byBkYiBkZW5pZWQuIFVzZXIgZG9lcyBub3QgaGF2ZSBwZXJtaXNzaW9u" +
		"cy4gQ29uZmlybSBkYXRhYmFzZSB1c2VyIGFuZCBuYW1lLg=="))
	f.Add(unverifiedBase64Bytes("DAAAAANzaG93IHRhYmxlcw=="))
	f.Add(unverifiedBase64Bytes("QQAAAP9RBHg1MDk6IGNlcnRpZmljYXRlIGlzIHZhbGlkIGZvciBsb2NhbGhvc3QsIG5vdCBhYmMu" +
		"ZXhhbXBsZS50ZXN0"))
	f.Add(unverifiedBase64Bytes("IAAAAP9RBGludmFsaWQgc2VydmVyIENBIGNlcnRpZmljYXRl"))
	f.Add(unverifiedBase64Bytes("QQAAAP9RBHg1MDk6IGNlcnRpZmljYXRlIGlzIHZhbGlkIGZvciBiYWQuZXhhbXBsZS50ZXN0LCBu" +
		"b3QgbG9jYWxob3N0"))
	f.Add([]byte{0x1, 0x0, 0x0, 0x0, 0xe})

	f.Fuzz(func(t *testing.T, packet []byte) {
		r := bytes.NewReader(packet)
		require.NotPanics(t, func() {
			_, _ = ParsePacket(r)
		})
	})
}

func FuzzFetchMySQLVersion(f *testing.F) {
	f.Add([]byte("00000"))
	f.Add(unverifiedBase64Bytes("bQAAAAo1LjUuNS0xMC44LjItTWFyaWFEQi0xOjEwLjguMittYXJpYX5mb2NhbAAEAAAANkdCVGJu" +
		"IloA/v8tAgD/wRUAAAAAAAAdAAAAIzxGKj44QHZWKGM9AG15c3FsX25hdGl2ZV9wYXNzd29yZAA="))
	f.Add(unverifiedBase64Bytes("bgAAAf9pBEhvc3QgJzcwLjE0NC4xMjguNDQnIGlzIGJsb2NrZWQgYmVjYXVzZSBvZiBtYW55IGNv" +
		"bm5lY3Rpb24gZXJyb3JzOyB1bmJsb2NrIHdpdGggJ21hcmlhZGItYWRtaW4gZmx1c2gtaG9zdHMn"))
	f.Add(unverifiedBase64Bytes("RAAAAAoAFCcAAHh4Y31HXjVsAA2qIQAAOAAVAAAAAAAAAAAAADEoIyJXI2R+MWlvawBteXNxbF9u" +
		"YXRpdmVfcGFzc3dvcmQA"))
	f.Add(unverifiedBase64Bytes("SgAAAAo4LjAuMTIAICcAAFUqTCQvPDllAA2qIQAAOAAVAAAAAAAAAAAAAGoySWlWaVdlM0tsagBt" +
		"eXNxbF9uYXRpdmVfcGFzc3dvcmQA"))
	f.Add(unverifiedBase64Bytes("RAAAAAoAIycAAH1pZFUxW2pCAA2qIQAAOAAVAAAAAAAAAAAAACZDTWlrNyk/ZlM8NABteXNxbF9u" +
		"YXRpdmVfcGFzc3dvcmQA"))
	f.Add(unverifiedBase64Bytes("SgAAAAo4LjAuMzMAGgAAAE97MgQmOFsMAP///wIA/98VAAAAAAAAAAAAAAFaBgdXT35odlQWMABj" +
		"YWNoaW5nX3NoYTJfcGFzc3dvcmQA"))
	f.Add(unverifiedBase64Bytes("RAAAAAoAJycAAB9VJR5QOSZyAA2qIQAAOAAVAAAAAAAAAAAAACdGZV8gYitPeCgqNQBteXNxbF9u" +
		"YXRpdmVfcGFzc3dvcmQA"))
	f.Add(unverifiedBase64Bytes("RAAAAAoAKScAAHBJOVkydiZ5AA2qIQAAOAAVAAAAAAAAAAAAAGZEbzAzOmNxZCBZMgBteXNxbF9u" +
		"YXRpdmVfcGFzc3dvcmQA"))
	f.Add(unverifiedBase64Bytes("RAAAAAoAKicAADppTGNDYkdZAA2qIQAAOAAVAAAAAAAAAAAAAFJZWilCNSs2aStwOQBteXNxbF9u" +
		"YXRpdmVfcGFzc3dvcmQA"))
	f.Add(unverifiedBase64Bytes("RAAAAAoALScAAH5NQx8+HihGAA2qIQAAOAAVAAAAAAAAAAAAAFlsfnpxIEU5KHdeSgBteXNxbF9u" +
		"YXRpdmVfcGFzc3dvcmQA"))
	f.Add(unverifiedBase64Bytes("RAAAAAoALicAAGRDRX5xRT54AA2qIQAAOAAVAAAAAAAAAAAAAFt9eHxtLnE6OTRfbgBteXNxbF9u" +
		"YXRpdmVfcGFzc3dvcmQA"))
	f.Add(unverifiedBase64Bytes("RAAAAAoAMScAACM6aSY/WD4oAA2qIQAAOAAVAAAAAAAAAAAAACkfYEpWKkMnXmpYRgBteXNxbF9u" +
		"YXRpdmVfcGFzc3dvcmQA"))
	f.Add(unverifiedBase64Bytes("RAAAAAoAMycAAG5HTVZBPDZXAA2qIQAAOAAVAAAAAAAAAAAAAC1wNjNHO0E5KllXcQBteXNxbF9u" +
		"YXRpdmVfcGFzc3dvcmQA"))
	f.Add(unverifiedBase64Bytes("RAAAAAoANScAAGAxbV1nL0xVAA2qIQAAOAAVAAAAAAAAAAAAAGBPeC00ICN6KGI6NQBteXNxbF9u" +
		"YXRpdmVfcGFzc3dvcmQA"))
	f.Add(unverifiedBase64Bytes("RAAAAAoAQScAAGEpL0JWQGtwAA2qIQAAOAAVAAAAAAAAAAAAACpeTD9tIDQ5a2xXXQBteXNxbF9u" +
		"YXRpdmVfcGFzc3dvcmQA"))
	f.Add(unverifiedBase64Bytes("RAAAAAoARCcAAGkwIjBNUkEvAA2qIQAAOAAVAAAAAAAAAAAAAEs4UUFpWmhVT0NMRABteXNxbF9u" +
		"YXRpdmVfcGFzc3dvcmQA"))
	f.Add(unverifiedBase64Bytes("SgAAAAo4LjAuMzMAGwAAAEsqGCYnIFZIAP///wIA/98VAAAAAAAAAAAAAHBbHyV6PFVpF1hrWABj" +
		"YWNoaW5nX3NoYTJfcGFzc3dvcmQA"))
	f.Add(unverifiedBase64Bytes("RAAAAAoASCcAAFZoeFFPSCs4AA2qIQAAOAAVAAAAAAAAAAAAAGtZeH10akN+Jk0ofQBteXNxbF9u" +
		"YXRpdmVfcGFzc3dvcmQA"))
	f.Add(unverifiedBase64Bytes("RAAAAAoATycAAGlhaXhPN1ZiAA2qIQAAOAAVAAAAAAAAAAAAADIefW8rRilKezE6cABteXNxbF9u" +
		"YXRpdmVfcGFzc3dvcmQA"))
	f.Add(unverifiedBase64Bytes("RAAAAAoAUicAAEd2dERHalsoAA2qIQAAOAAVAAAAAAAAAAAAACZGIzJVWnUseTsocgBteXNxbF9u" +
		"YXRpdmVfcGFzc3dvcmQA"))

	f.Fuzz(func(t *testing.T, packet []byte) {
		ctx := context.Background()
		r := bytes.NewReader(packet)

		require.NotPanics(t, func() {
			_, _ = FetchMySQLVersionInternal(ctx, func(ctx context.Context, network, address string) (net.Conn, error) {
				return &buffTestReader{reader: r}, nil
			}, "")
		})
	})
}

func FuzzIsHandshakeV10Packet(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0})
	f.Add([]byte{0, 0, 0, 0, 0})
	f.Add([]byte{0, 0, 0, 0, 10})
	f.Fuzz(func(t *testing.T, packet []byte) {
		ctx := context.Background()
		conn := NewBufferedConn(ctx, &buffTestReader{reader: bytes.NewReader(packet)})
		require.NotPanics(t, func() {
			_, _ = IsHandshakeV10Packet(conn)
		})
		require.NotNil(t, conn)
		got, err := io.ReadAll(conn)
		require.NoError(t, err)
		require.Equal(t, got, packet, "it should only peek into the conn without consuming bytes")
	})
}

// buffTestReader is a fake reader used for test where read only
// net.Conn is needed.
type buffTestReader struct {
	reader *bytes.Reader
	net.Conn
}

func (r *buffTestReader) Read(b []byte) (int, error) {
	return r.reader.Read(b)
}

func (r *buffTestReader) Close() error {
	return nil
}
