//go:build go1.16
// +build go1.16

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package runtime

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/internal/shared"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/internal/errorinfo"
)

// bodyDownloadPolicy creates a policy object that downloads the response's body to a []byte.
func bodyDownloadPolicy(req *policy.Request) (*http.Response, error) {
	resp, err := req.Next()
	if err != nil {
		return resp, err
	}
	var opValues shared.BodyDownloadPolicyOpValues
	// don't skip downloading error response bodies
	if req.OperationValue(&opValues); opValues.Skip && resp.StatusCode < 400 {
		return resp, err
	}
	// Either bodyDownloadPolicyOpValues was not specified (so skip is false)
	// or it was specified and skip is false: don't skip downloading the body
	b, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return resp, newBodyDownloadError(err, req)
	}
	resp.Body = &nopClosingBytesReader{s: b}
	return resp, err
}

type bodyDownloadError struct {
	err error
}

func newBodyDownloadError(err error, req *policy.Request) error {
	// on failure, only retry the request for idempotent operations.
	// we currently identify them as DELETE, GET, and PUT requests.
	if m := strings.ToUpper(req.Raw().Method); m == http.MethodDelete || m == http.MethodGet || m == http.MethodPut {
		// error is safe for retry
		return err
	}
	// wrap error to avoid retries
	return &bodyDownloadError{
		err: err,
	}
}

func (b *bodyDownloadError) Error() string {
	return fmt.Sprintf("body download policy: %s", b.err.Error())
}

func (b *bodyDownloadError) NonRetriable() {
	// marker method
}

func (b *bodyDownloadError) Unwrap() error {
	return b.err
}

var _ errorinfo.NonRetriable = (*bodyDownloadError)(nil)

// nopClosingBytesReader is an io.ReadSeekCloser around a byte slice.
// It also provides direct access to the byte slice.
type nopClosingBytesReader struct {
	s []byte
	i int64
}

// Bytes returns the underlying byte slice.
func (r *nopClosingBytesReader) Bytes() []byte {
	return r.s
}

// Close implements the io.Closer interface.
func (*nopClosingBytesReader) Close() error {
	return nil
}

// Read implements the io.Reader interface.
func (r *nopClosingBytesReader) Read(b []byte) (n int, err error) {
	if r.i >= int64(len(r.s)) {
		return 0, io.EOF
	}
	n = copy(b, r.s[r.i:])
	r.i += int64(n)
	return
}

// Set replaces the existing byte slice with the specified byte slice and resets the reader.
func (r *nopClosingBytesReader) Set(b []byte) {
	r.s = b
	r.i = 0
}

// Seek implements the io.Seeker interface.
func (r *nopClosingBytesReader) Seek(offset int64, whence int) (int64, error) {
	var i int64
	switch whence {
	case io.SeekStart:
		i = offset
	case io.SeekCurrent:
		i = r.i + offset
	case io.SeekEnd:
		i = int64(len(r.s)) + offset
	default:
		return 0, errors.New("nopClosingBytesReader: invalid whence")
	}
	if i < 0 {
		return 0, errors.New("nopClosingBytesReader: negative position")
	}
	r.i = i
	return i, nil
}
