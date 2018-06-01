/*
Copyright 2015-2017 Gravitational, Inc.

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

/*Package roundtrip provides convenient functions for building HTTP client wrappers
and providing functions for server responses.

 import (
    "github.com/gravitational/roundtrip"
 )

 type MyClient struct {
     roundtrip.Client // you can embed roundtrip client
 }

 func NewClient(addr, version string) (*MyClient, error) {
     c, err := roundtrip.NewClient(addr, version)
     if err != nil {
         return nil, err
     }
     return &MyClient{*c}, nil
 }
*/
package roundtrip

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
)

// ClientParam specifies functional argument for client
type ClientParam func(c *Client) error

// Tracer sets a request tracer constructor
func Tracer(newTracer NewTracer) ClientParam {
	return func(c *Client) error {
		c.newTracer = newTracer
		return nil
	}
}

// HTTPClient is a functional parameter that sets the internal
// HTTPClient of the roundtrip client wrapper
func HTTPClient(h *http.Client) ClientParam {
	return func(c *Client) error {
		c.client = h
		return nil
	}
}

// BasicAuth sets username and password for HTTP client
func BasicAuth(username, password string) ClientParam {
	return func(c *Client) error {
		c.auth = &basicAuth{username: username, password: password}
		return nil
	}
}

// BearerAuth sets token for HTTP client
func BearerAuth(token string) ClientParam {
	return func(c *Client) error {
		c.auth = &bearerAuth{token: token}
		return nil
	}
}

// CookieJar sets HTTP cookie jar for this client
func CookieJar(jar http.CookieJar) ClientParam {
	return func(c *Client) error {
		c.jar = jar
		return nil
	}
}

// SanitizerEnabled will enable the input sanitizer which passes the URL
// path through a strict whitelist.
func SanitizerEnabled(sanitizerEnabled bool) ClientParam {
	return func(c *Client) error {
		c.sanitizerEnabled = sanitizerEnabled
		return nil
	}
}

// Client is a wrapper holding HTTP client. It hold target server address and a version prefix,
// and provides common features for building HTTP client wrappers.
type Client struct {
	// addr is target server address
	addr string
	// v is a version prefix
	v string
	// client is a private http.Client instance
	client *http.Client
	// auth tells client to use HTTP auth on every request
	auth fmt.Stringer
	// jar is a set of cookies passed with requests
	jar http.CookieJar
	// newTracer creates new request tracer
	newTracer NewTracer
	// sanitizerEnabled will enable the input sanitizer which passes the URL
	// path through a strict whitelist.
	sanitizerEnabled bool
}

// NewClient returns a new instance of roundtrip.Client, or nil and error
//
//   c, err := NewClient("http://localhost:8080", "v1")
//   if err != nil {
//       // handle error
//   }
//
func NewClient(addr, v string, params ...ClientParam) (*Client, error) {
	c := &Client{
		addr:   addr,
		v:      v,
		client: &http.Client{},
	}
	for _, p := range params {
		if err := p(c); err != nil {
			return nil, err
		}
	}
	if c.jar != nil {
		c.client.Jar = c.jar
	}
	if c.newTracer == nil {
		c.newTracer = NewNopTracer
	}
	return c, nil
}

// HTTPClient returns underlying http.Client
func (c *Client) HTTPClient() *http.Client {
	return c.client
}

// Endpoint returns a URL constructed from parts and version appended, e.g.
//
// c.Endpoint("user", "john") // returns "/v1/users/john"
//
func (c *Client) Endpoint(params ...string) string {
	if c.v != "" {
		return fmt.Sprintf("%s/%s/%s", c.addr, c.v, strings.Join(params, "/"))
	}
	return fmt.Sprintf("%s/%s", c.addr, strings.Join(params, "/"))
}

// PostForm posts urlencoded form with values and returns the result
//
// c.PostForm(c.Endpoint("users"), url.Values{"name": []string{"John"}})
//
func (c *Client) PostForm(endpoint string, vals url.Values, files ...File) (*Response, error) {
	// If the sanitizer is enabled, make sure the requested path is safe.
	if c.sanitizerEnabled {
		err := isPathSafe(endpoint)
		if err != nil {
			return nil, err
		}
	}

	return c.RoundTrip(func() (*http.Response, error) {
		if len(files) == 0 {
			req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(vals.Encode()))
			if err != nil {
				return nil, err
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			c.addAuth(req)
			return c.client.Do(req)
		}

		var buf bytes.Buffer
		buf.Grow(bufferSize)

		// Cache file reads in case we reach the memory limit
		// and need to rewind
		buffers := newBuffersFromFiles(files)
		writer := multipart.NewWriter(&limitWriter{&buf, bufferSize})
		err := writeForm(writer, vals, buffers...)
		writer.Close()
		if err != nil && err != errShortWrite {
			return nil, err
		}

		if err == errShortWrite {
			// Switch to io.Pipe as the data is larger than the memory limit
			for i := range buffers {
				buffers[i].rewind()
			}
			return c.writeWithPipe(endpoint, vals, buffers...)
		}

		req, err := http.NewRequest(http.MethodPost, endpoint, &buf)
		if err != nil {
			return nil, err
		}
		c.addAuth(req)
		req.Header.Set("Content-Type",
			fmt.Sprintf(`multipart/form-data;boundary="%v"`, writer.Boundary()))
		return c.client.Do(req)
	})
}

// PostJSON posts JSON "application/json" encoded request body
//
// c.PostJSON(c.Endpoint("users"), map[string]string{"name": "alice@example.com"})
//
func (c *Client) PostJSON(endpoint string, data interface{}) (*Response, error) {
	// If the sanitizer is enabled, make sure the requested path is safe.
	if c.sanitizerEnabled {
		err := isPathSafe(endpoint)
		if err != nil {
			return nil, err
		}
	}

	tracer := c.newTracer()
	return tracer.Done(c.RoundTrip(func() (*http.Response, error) {
		data, err := json.Marshal(data)
		req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(data))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		c.addAuth(req)
		tracer.Start(req)
		return c.client.Do(req)
	}))
}

// PutJSON posts JSON "application/json" encoded request body and "PUT" method
//
// c.PutJSON(c.Endpoint("users"), map[string]string{"name": "alice@example.com"})
//
func (c *Client) PutJSON(endpoint string, data interface{}) (*Response, error) {
	// If the sanitizer is enabled, make sure the requested path is safe.
	if c.sanitizerEnabled {
		err := isPathSafe(endpoint)
		if err != nil {
			return nil, err
		}
	}

	tracer := c.newTracer()
	return tracer.Done(c.RoundTrip(func() (*http.Response, error) {
		data, err := json.Marshal(data)
		req, err := http.NewRequest(http.MethodPut, endpoint, bytes.NewBuffer(data))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		c.addAuth(req)
		tracer.Start(req)
		return c.client.Do(req)
	}))
}

// PatchJSON posts JSON "application/json" encoded request body and "PATCH" method
//
// c.PatchJSON(c.Endpoint("users"), map[string]string{"name": "alice@example.com"})
//
func (c *Client) PatchJSON(endpoint string, data interface{}) (*Response, error) {
	// If the sanitizer is enabled, make sure the requested path is safe.
	if c.sanitizerEnabled {
		err := isPathSafe(endpoint)
		if err != nil {
			return nil, err
		}
	}

	tracer := c.newTracer()
	return tracer.Done(c.RoundTrip(func() (*http.Response, error) {
		data, err := json.Marshal(data)
		req, err := http.NewRequest(http.MethodPatch, endpoint, bytes.NewBuffer(data))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		c.addAuth(req)
		tracer.Start(req)
		return c.client.Do(req)
	}))
}

// Delete executes DELETE request to the endpoint with no body
//
// re, err := c.Delete(c.Endpoint("users", "id1"))
//
func (c *Client) Delete(endpoint string) (*Response, error) {
	// If the sanitizer is enabled, make sure the requested path is safe.
	if c.sanitizerEnabled {
		err := isPathSafe(endpoint)
		if err != nil {
			return nil, err
		}
	}

	tracer := c.newTracer()
	return tracer.Done(c.RoundTrip(func() (*http.Response, error) {
		req, err := http.NewRequest(http.MethodDelete, endpoint, nil)
		if err != nil {
			return nil, err
		}
		c.addAuth(req)
		tracer.Start(req)
		return c.client.Do(req)
	}))
}

// DeleteWithParams executes DELETE request to the endpoint with optional query arguments
//
// re, err := c.DeleteWithParams(c.Endpoint("users", "id1"), url.Values{"force": []string{"true"}})
//
func (c *Client) DeleteWithParams(endpoint string, params url.Values) (*Response, error) {
	baseURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	baseURL.RawQuery = params.Encode()
	return c.Delete(baseURL.String())
}

// Get executes GET request to the server endpoint with optional query arguments passed in params
//
// re, err := c.Get(c.Endpoint("users"), url.Values{"name": []string{"John"}})
//
func (c *Client) Get(endpoint string, params url.Values) (*Response, error) {
	// If the sanitizer is enabled, make sure the requested path is safe.
	if c.sanitizerEnabled {
		err := isPathSafe(endpoint)
		if err != nil {
			return nil, err
		}
	}

	baseUrl, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	baseUrl.RawQuery = params.Encode()
	tracer := c.newTracer()
	return tracer.Done(c.RoundTrip(func() (*http.Response, error) {
		req, err := http.NewRequest(http.MethodGet, baseUrl.String(), nil)
		if err != nil {
			return nil, err
		}
		c.addAuth(req)
		tracer.Start(req)
		return c.client.Do(req)
	}))
}

// GetFile executes get request and returns a file like object
//
// f, err := c.GetFile("files", "report.txt") // returns "/v1/files/report.txt"
//
func (c *Client) GetFile(endpoint string, params url.Values) (*FileResponse, error) {
	// If the sanitizer is enabled, make sure the requested path is safe.
	if c.sanitizerEnabled {
		err := isPathSafe(endpoint)
		if err != nil {
			return nil, err
		}
	}

	baseUrl, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	baseUrl.RawQuery = params.Encode()
	req, err := http.NewRequest(http.MethodGet, baseUrl.String(), nil)
	if err != nil {
		return nil, err
	}
	c.addAuth(req)
	tracer := c.newTracer()
	tracer.Start(req)
	re, err := c.client.Do(req)
	if err != nil {
		tracer.Done(nil, err)
		return nil, err
	}
	tracer.Done(&Response{code: re.StatusCode}, err)
	return &FileResponse{
		code:    re.StatusCode,
		headers: re.Header,
		body:    re.Body,
	}, nil
}

// ReadSeekCloser implements all three of Seeker, Closer and Reader interfaces
type ReadSeekCloser interface {
	io.ReadSeeker
	io.Closer
}

// OpenFile opens file using HTTP protocol and uses `Range` headers
// to seek to various positions in the file, this means that server
// has to support the flags `Range` and `Content-Range`
func (c *Client) OpenFile(endpoint string, params url.Values) (ReadSeekCloser, error) {
	// If the sanitizer is enabled, make sure the requested path is safe.
	if c.sanitizerEnabled {
		err := isPathSafe(endpoint)
		if err != nil {
			return nil, err
		}
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	u.RawQuery = params.Encode()

	return newSeeker(c, u.String())
}

// RoundTripFn inidicates any function that can be passed to RoundTrip
// it should return HTTP response or error in case of error
type RoundTripFn func() (*http.Response, error)

// RoundTrip collects response and error assuming fn has done
// HTTP roundtrip
func (c *Client) RoundTrip(fn RoundTripFn) (*Response, error) {
	re, err := fn()
	if err != nil {
		return nil, err
	}
	defer re.Body.Close()
	buf := &bytes.Buffer{}
	_, err = io.Copy(buf, re.Body)
	if err != nil {
		return nil, err
	}
	return &Response{
		code:    re.StatusCode,
		headers: re.Header,
		body:    buf,
		cookies: re.Cookies(),
	}, nil
}

// SetAuthHeader sets client's authorization headers if client
// was configured to work with authorization
func (c *Client) SetAuthHeader(h http.Header) {
	if c.auth != nil {
		h.Set("Authorization", c.auth.String())
	}
}

func (c *Client) addAuth(r *http.Request) {
	if c.auth != nil {
		r.Header.Set("Authorization", c.auth.String())
	}
}

func (c *Client) writeWithPipe(endpoint string, vals url.Values, buffers ...fileBuffer) (*http.Response, error) {
	r, w := io.Pipe()
	writer := multipart.NewWriter(w)

	go func() {
		err := writeForm(writer, vals, buffers...)
		writer.Close()
		w.CloseWithError(err)
	}()

	req, err := http.NewRequest(http.MethodPost, endpoint, r)
	if err != nil {
		r.Close()
		return nil, err
	}

	c.addAuth(req)
	req.Header.Set("Content-Type", fmt.Sprintf(`multipart/form-data;boundary="%v"`, writer.Boundary()))
	return c.client.Do(req)
}

// Response indicates HTTP server response
type Response struct {
	code    int
	headers http.Header
	body    *bytes.Buffer
	cookies []*http.Cookie
}

// Cookies returns a list of cookies set by server
func (r *Response) Cookies() []*http.Cookie {
	return r.cookies
}

// Code returns HTTP response status code
func (r *Response) Code() int {
	return r.code
}

// Headers returns http.Header dictionary with response headers
func (r *Response) Headers() http.Header {
	return r.headers
}

// Reader returns reader with HTTP response body
func (r *Response) Reader() io.Reader {
	return r.body
}

// Bytes reads all http response body bytes in memory and returns the result
func (r *Response) Bytes() []byte {
	return r.body.Bytes()
}

// File is a file-like object that can be posted to the files
type File struct {
	Name     string
	Filename string
	Reader   io.Reader
}

// FileResponse indicates HTTP server file response
type FileResponse struct {
	code    int
	headers http.Header
	body    io.ReadCloser
}

// FileName returns HTTP file name
func (r *FileResponse) FileName() string {
	value := r.headers.Get("Content-Disposition")
	if len(value) == 0 {
		return ""
	}
	_, params, err := mime.ParseMediaType(value)
	if err != nil {
		return ""
	}
	return params["filename"]
}

// Code returns HTTP response status code
func (r *FileResponse) Code() int {
	return r.code
}

// Headers returns http.Header dictionary with response headers
func (r *FileResponse) Headers() http.Header {
	return r.headers
}

// Body returns reader with HTTP response body
func (r *FileResponse) Body() io.ReadCloser {
	return r.body
}

// Close closes internal response body
func (r *FileResponse) Close() error {
	return r.body.Close()
}

type basicAuth struct {
	username string
	password string
}

func (b *basicAuth) String() string {
	auth := b.username + ":" + b.password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
}

type bearerAuth struct {
	token string
}

func (b *bearerAuth) String() string {
	return "Bearer " + b.token
}

func writeForm(writer *multipart.Writer, vals url.Values, files ...fileBuffer) error {
	// write simple fields
	for name, vals := range vals {
		for _, val := range vals {
			if err := writer.WriteField(name, val); err != nil {
				return err
			}
		}
	}

	// add files
	for _, file := range files {
		output, err := file.create(writer)
		if err != nil {
			return err
		}
		_, err = io.Copy(output, file)
		if err != nil {
			return err
		}
	}
	return nil
}

// newBuffersFromFiles wraps the specified files with a reader
// that caches data into a memory buffer
func newBuffersFromFiles(files []File) []fileBuffer {
	buffers := make([]fileBuffer, 0, len(files))
	for _, file := range files {
		buffers = append(buffers, newFileBuffer(file))
	}
	return buffers
}

// newFileBuffer creates a buffer for reading from the specified File file
func newFileBuffer(file File) fileBuffer {
	buf := &bytes.Buffer{}
	return fileBuffer{
		Reader: io.TeeReader(file.Reader, buf),
		File:   file,
		cache:  buf,
	}
}

// Read reads data from the underlying reader into the specified array p
func (r fileBuffer) Read(p []byte) (n int, err error) {
	return r.Reader.Read(p)
}

func (r *fileBuffer) create(w *multipart.Writer) (io.Writer, error) {
	return w.CreateFormFile(r.Name, r.Filename)
}

// rewind resets this fileBuffer to read from the beginning
func (r *fileBuffer) rewind() {
	r.Reader = io.MultiReader(r.cache, r.File.Reader)
}

// fileBuffer is a File wrapper that buffers data from the specified File
type fileBuffer struct {
	io.Reader
	File
	cache *bytes.Buffer
}

func (r *limitWriter) Write(p []byte) (n int, err error) {
	if int64(len(p)) > r.maxBytes {
		p = p[:r.maxBytes]
		err = errShortWrite
	}
	var errWrite error
	n, errWrite = r.Writer.Write(p)
	r.maxBytes -= int64(n)
	if errWrite != nil {
		err = errWrite
	}
	return n, err
}

// limitWriter is an io.Writer that aborts with errShortWrite
// if more than maxBytes bytes are written
type limitWriter struct {
	io.Writer
	maxBytes int64
}

var errShortWrite = errors.New("short write")

// bufferSize specifies the upper bound on the data before PostForm switches
// to io.Pipe to avoid reading larger files into memory
const bufferSize = 1024 << 10 // 1 MiB
