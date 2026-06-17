/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

// Package har provides a reusable HAR 1.2 builder that accumulates HTTP
// recording audit events and encodes them as a HAR (HTTP Archive) document.
package har

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
)

// HAR 1.2 types (https://w3c.github.io/web-performance/specs/HAR/Overview.html)

type Root struct {
	Log Log `json:"log"`
}

type Log struct {
	Version string  `json:"version"`
	Creator Creator `json:"creator"`
	Entries []Entry `json:"entries"`
}

type Creator struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Entry struct {
	StartedDateTime string   `json:"startedDateTime"`
	Time            float64  `json:"time"`
	Request         Request  `json:"request"`
	Response        Response `json:"response"`
	Timings         Timings  `json:"timings"`
}

type Request struct {
	Method      string      `json:"method"`
	URL         string      `json:"url"`
	HTTPVersion string      `json:"httpVersion"`
	Headers     []NameValue `json:"headers"`
	QueryString []NameValue `json:"queryString"`
	PostData    *PostData   `json:"postData,omitempty"`
	HeadersSize int         `json:"headersSize"` // -1 = not tracked
	BodySize    int         `json:"bodySize"`    // -1 = not tracked
}

type Response struct {
	Status      int         `json:"status"`
	StatusText  string      `json:"statusText"`
	HTTPVersion string      `json:"httpVersion"`
	Headers     []NameValue `json:"headers"`
	Content     Content     `json:"content"`
	RedirectURL string      `json:"redirectURL"`
	HeadersSize int         `json:"headersSize"` // -1 = not tracked
	BodySize    int         `json:"bodySize"`    // -1 = not tracked
}

type NameValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type PostData struct {
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
	Encoding string `json:"encoding,omitempty"` // "base64" for binary content
}

type Content struct {
	Size     int    `json:"size"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text,omitempty"`
	Encoding string `json:"encoding,omitempty"` // "base64" for binary content
}

type Timings struct {
	Send    float64 `json:"send"`
	Wait    float64 `json:"wait"`
	Receive float64 `json:"receive"`
}

type bodyChunk struct {
	index int64
	data  []byte
}

type pendingEntry struct {
	request    *apievents.AppSessionHTTPRequest
	reqChunks  []bodyChunk
	response   *apievents.AppSessionHTTPResponse
	respChunks []bodyChunk
}

// Builder accumulates HTTP recording audit events and can encode them as a
// HAR 1.2 document. Events are emitted in first-seen request order.
type Builder struct {
	pending map[string]*pendingEntry
	order   []string // request IDs in first-seen order
}

// NewBuilder returns a new, empty Builder.
func NewBuilder() *Builder {
	return &Builder{pending: map[string]*pendingEntry{}}
}

// Add incorporates an audit event into the builder. It accepts the four
// HTTP-recording event types and silently ignores all other event types.
func (b *Builder) Add(evt apievents.AuditEvent) {
	switch e := evt.(type) {
	case *apievents.AppSessionHTTPRequest:
		if _, seen := b.pending[e.RequestId]; !seen {
			b.order = append(b.order, e.RequestId)
			b.pending[e.RequestId] = &pendingEntry{}
		}
		b.pending[e.RequestId].request = e

	case *apievents.AppSessionHTTPRequestBodyChunk:
		if p, ok := b.pending[e.RequestId]; ok {
			p.reqChunks = append(p.reqChunks, bodyChunk{e.ChunkIndex, e.Data})
		}

	case *apievents.AppSessionHTTPResponse:
		if p, ok := b.pending[e.RequestId]; ok {
			p.response = e
		}

	case *apievents.AppSessionHTTPResponseBodyChunk:
		if p, ok := b.pending[e.RequestId]; ok {
			p.respChunks = append(p.respChunks, bodyChunk{e.ChunkIndex, e.Data})
		}
	}
}

// Encode writes a HAR 1.2 JSON document (2-space indented) to w containing
// all accumulated HTTP exchanges in first-seen request order.
func (b *Builder) Encode(w io.Writer) error {
	entries := make([]Entry, 0, len(b.order))
	for _, rid := range b.order {
		p := b.pending[rid]
		if p.request == nil {
			continue
		}
		entries = append(entries, buildHAREntry(p))
	}

	root := Root{
		Log: Log{
			Version: "1.2",
			Creator: Creator{Name: "Teleport", Version: teleport.Version},
			Entries: entries,
		},
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return trace.Wrap(enc.Encode(root))
}

// Parse decodes a HAR 1.2 document.
func Parse(data []byte) (Root, error) {
	var root Root
	if err := json.Unmarshal(data, &root); err != nil {
		return Root{}, trace.Wrap(err)
	}
	return root, nil
}

// EntryMeta is lightweight metadata for one HAR entry, used to render an
// exchange list without loading bodies.
type EntryMeta struct {
	Index            int
	Method           string
	URL              string
	StatusCode       int
	RequestBodySize  int
	ResponseBodySize int
	ResponseMIMEType string
	StartedDateTime  string
	TotalTimeMs      float64
}

// Index returns lightweight per-entry metadata for the document's entries, in
// chronological order. It carries no request/response bodies.
func (r Root) Index() []EntryMeta {
	metas := make([]EntryMeta, 0, len(r.Log.Entries))
	for i, e := range r.Log.Entries {
		var reqBodySize int
		if e.Request.PostData != nil {
			reqBodySize = len(e.Request.PostData.Text)
		}
		metas = append(metas, EntryMeta{
			Index:            i,
			Method:           e.Request.Method,
			URL:              e.Request.URL,
			StatusCode:       e.Response.Status,
			RequestBodySize:  reqBodySize,
			ResponseBodySize: e.Response.Content.Size,
			ResponseMIMEType: e.Response.Content.MimeType,
			StartedDateTime:  e.StartedDateTime,
			TotalTimeMs:      e.Time,
		})
	}
	return metas
}

func buildHAREntry(p *pendingEntry) Entry {
	req := p.request

	reqMIME := harHeaderMIMEType(req.Headers)
	reqBody := assembleHARBody(p.reqChunks)

	var postData *PostData
	if len(reqBody) > 0 {
		text, enc := harEncodeBody(reqBody, reqMIME)
		postData = &PostData{
			MimeType: reqMIME,
			Text:     text,
			Encoding: enc,
		}
	}

	harReq := Request{
		Method:      req.Method,
		URL:         req.Url,
		HTTPVersion: req.HttpVersion,
		Headers:     harConvertHeaders(req.Headers),
		QueryString: harParseQueryString(req.RawQuery),
		PostData:    postData,
		HeadersSize: -1,
		BodySize:    -1,
	}

	// Response may be absent if the session ended mid-request.
	var harResp Response
	var timings Timings
	if p.response != nil {
		resp := p.response
		respMIME := harHeaderMIMEType(resp.Headers)
		respBody := assembleHARBody(p.respChunks)

		var contentText, contentEnc string
		if len(respBody) > 0 {
			contentText, contentEnc = harEncodeBody(respBody, respMIME)
		}

		// Prefer the canonical reason phrase; fall back to stripping the
		// leading "NNN " from the stored StatusText for non-standard codes.
		statusText := http.StatusText(int(resp.StatusCode))
		if statusText == "" {
			if idx := strings.IndexByte(resp.StatusText, ' '); idx != -1 {
				statusText = resp.StatusText[idx+1:]
			} else {
				statusText = resp.StatusText
			}
		}

		harResp = Response{
			Status:      int(resp.StatusCode),
			StatusText:  statusText,
			HTTPVersion: resp.HttpVersion,
			Headers:     harConvertHeaders(resp.Headers),
			Content: Content{
				Size:     len(respBody),
				MimeType: respMIME,
				Text:     contentText,
				Encoding: contentEnc,
			},
			RedirectURL: harHeaderValue(resp.Headers, "Location"),
			HeadersSize: -1,
			BodySize:    -1,
		}
		timings = Timings{
			Send:    float64(resp.SendTimeMs),
			Wait:    float64(resp.WaitTimeMs),
			Receive: float64(resp.ReceiveTimeMs),
		}
	}

	return Entry{
		StartedDateTime: req.GetTime().UTC().Format("2006-01-02T15:04:05.000Z"),
		Time:            timings.Send + timings.Wait + timings.Receive,
		Request:         harReq,
		Response:        harResp,
		Timings:         timings,
	}
}

// assembleHARBody sorts chunks by index and concatenates their data.
func assembleHARBody(chunks []bodyChunk) []byte {
	if len(chunks) == 0 {
		return nil
	}
	slices.SortFunc(chunks, func(a, b bodyChunk) int {
		if a.index < b.index {
			return -1
		}
		if a.index > b.index {
			return 1
		}
		return 0
	})
	var buf []byte
	for _, c := range chunks {
		buf = append(buf, c.data...)
	}
	return buf
}

// harEncodeBody returns body data as a string. Text MIME types are returned
// as-is (UTF-8); everything else is base64-encoded.
func harEncodeBody(data []byte, mimeType string) (text, encoding string) {
	if isTextMIMEType(mimeType) {
		return string(data), ""
	}
	return base64.StdEncoding.EncodeToString(data), "base64"
}

// isTextMIMEType reports whether mimeType represents human-readable text.
func isTextMIMEType(mimeType string) bool {
	if mimeType == "" {
		return false
	}
	base := strings.TrimSpace(strings.ToLower(strings.SplitN(mimeType, ";", 2)[0]))
	if strings.HasPrefix(base, "text/") {
		return true
	}
	switch base {
	case "application/json",
		"application/xml",
		"application/javascript",
		"application/x-javascript",
		"application/ld+json",
		"application/graphql",
		"application/x-www-form-urlencoded",
		"application/manifest+json",
		"application/xhtml+xml",
		"application/rss+xml",
		"application/atom+xml":
		return true
	}
	return false
}

func harConvertHeaders(headers []*apievents.HTTPHeader) []NameValue {
	out := make([]NameValue, 0, len(headers))
	for _, h := range headers {
		out = append(out, NameValue{Name: h.Name, Value: h.Value})
	}
	return out
}

func harParseQueryString(rawQuery string) []NameValue {
	vals, _ := url.ParseQuery(rawQuery)
	out := make([]NameValue, 0, len(vals))
	for k, vs := range vals {
		for _, v := range vs {
			out = append(out, NameValue{Name: k, Value: v})
		}
	}
	return out
}

// harHeaderMIMEType returns the base MIME type from a Content-Type header.
func harHeaderMIMEType(headers []*apievents.HTTPHeader) string {
	v := harHeaderValue(headers, "Content-Type")
	if v == "" {
		return ""
	}
	return strings.TrimSpace(strings.SplitN(v, ";", 2)[0])
}

// harHeaderValue returns the value of the first header matching name (case-insensitive).
func harHeaderValue(headers []*apievents.HTTPHeader, name string) string {
	for _, h := range headers {
		if strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	return ""
}
