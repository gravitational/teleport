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

package common

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
)

// HAR 1.2 types (https://w3c.github.io/web-performance/specs/HAR/Overview.html)

type harRoot struct {
	Log harLog `json:"log"`
}

type harLog struct {
	Version string     `json:"version"`
	Creator harCreator `json:"creator"`
	Entries []harEntry `json:"entries"`
}

type harCreator struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type harEntry struct {
	StartedDateTime string      `json:"startedDateTime"`
	Time            float64     `json:"time"`
	Request         harRequest  `json:"request"`
	Response        harResponse `json:"response"`
	Timings         harTimings  `json:"timings"`
}

type harRequest struct {
	Method      string         `json:"method"`
	URL         string         `json:"url"`
	HTTPVersion string         `json:"httpVersion"`
	Headers     []harNameValue `json:"headers"`
	QueryString []harNameValue `json:"queryString"`
	PostData    *harPostData   `json:"postData,omitempty"`
	HeadersSize int            `json:"headersSize"` // -1 = not tracked
	BodySize    int            `json:"bodySize"`    // -1 = not tracked
}

type harResponse struct {
	Status      int            `json:"status"`
	StatusText  string         `json:"statusText"`
	HTTPVersion string         `json:"httpVersion"`
	Headers     []harNameValue `json:"headers"`
	Content     harContent     `json:"content"`
	RedirectURL string         `json:"redirectURL"`
	HeadersSize int            `json:"headersSize"` // -1 = not tracked
	BodySize    int            `json:"bodySize"`    // -1 = not tracked
}

type harNameValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type harPostData struct {
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
	Encoding string `json:"encoding,omitempty"` // "base64" for binary content
}

type harContent struct {
	Size     int    `json:"size"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text,omitempty"`
	Encoding string `json:"encoding,omitempty"` // "base64" for binary content
}

type harTimings struct {
	Send    float64 `json:"send"`
	Wait    float64 `json:"wait"`
	Receive float64 `json:"receive"`
}

// pendingHAREntry accumulates events for a single HTTP exchange keyed by RequestId.
type pendingHAREntry struct {
	request    *apievents.AppSessionHTTPRequest
	reqChunks  []harBodyChunk
	response   *apievents.AppSessionHTTPResponse
	respChunks []harBodyChunk
}

type harBodyChunk struct {
	index int64
	data  []byte
}

// writeHAR reads HTTP recording events from the provided channels and writes a HAR 1.2 file to outputPath.
// The caller is responsible for creating the stream; the AppSessionStart event that identified this as
// an app session has already been consumed before this is called.
func writeHAR(ctx context.Context, evts <-chan apievents.AuditEvent, errs <-chan error, outputPath string, write func(format string, args ...any) (int, error)) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pending := map[string]*pendingHAREntry{}
	var order []string // request IDs in first-seen order
loop:
	for {
		select {
		case err := <-errs:
			return trace.Wrap(err)
		case <-ctx.Done():
			return ctx.Err()
		case evt, more := <-evts:
			if !more {
				break loop
			}
			switch e := evt.(type) {
			case *apievents.AppSessionHTTPRequest:
				if _, seen := pending[e.RequestId]; !seen {
					order = append(order, e.RequestId)
					pending[e.RequestId] = &pendingHAREntry{}
				}
				pending[e.RequestId].request = e

			case *apievents.AppSessionHTTPRequestBodyChunk:
				if p, ok := pending[e.RequestId]; ok {
					p.reqChunks = append(p.reqChunks, harBodyChunk{e.ChunkIndex, e.Data})
				}

			case *apievents.AppSessionHTTPResponse:
				if p, ok := pending[e.RequestId]; ok {
					p.response = e
				}

			case *apievents.AppSessionHTTPResponseBodyChunk:
				if p, ok := pending[e.RequestId]; ok {
					p.respChunks = append(p.respChunks, harBodyChunk{e.ChunkIndex, e.Data})
				}
			}
		}
	}

	entries := make([]harEntry, 0, len(order))
	for _, rid := range order {
		p := pending[rid]
		if p.request == nil {
			continue
		}
		entries = append(entries, buildHAREntry(p))
	}

	root := harRoot{
		Log: harLog{
			Version: "1.2",
			Creator: harCreator{Name: "Teleport", Version: teleport.Version},
			Entries: entries,
		},
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return trace.Wrap(err)
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if encErr := enc.Encode(root); encErr != nil {
		f.Close()
		return trace.Wrap(encErr)
	}
	if err := f.Close(); err != nil {
		return trace.Wrap(err)
	}

	if write != nil {
		write("wrote %v (%d entries)\n", outputPath, len(entries))
	}
	return nil
}

func buildHAREntry(p *pendingHAREntry) harEntry {
	req := p.request

	reqMIME := harHeaderMIMEType(req.Headers)
	reqBody := assembleHARBody(p.reqChunks)

	var postData *harPostData
	if len(reqBody) > 0 {
		text, enc := harEncodeBody(reqBody, reqMIME)
		postData = &harPostData{
			MimeType: reqMIME,
			Text:     text,
			Encoding: enc,
		}
	}

	harReq := harRequest{
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
	var harResp harResponse
	var timings harTimings
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

		harResp = harResponse{
			Status:      int(resp.StatusCode),
			StatusText:  statusText,
			HTTPVersion: resp.HttpVersion,
			Headers:     harConvertHeaders(resp.Headers),
			Content: harContent{
				Size:     len(respBody),
				MimeType: respMIME,
				Text:     contentText,
				Encoding: contentEnc,
			},
			RedirectURL: harHeaderValue(resp.Headers, "Location"),
			HeadersSize: -1,
			BodySize:    -1,
		}
		timings = harTimings{
			Send:    float64(resp.SendTimeMs),
			Wait:    float64(resp.WaitTimeMs),
			Receive: float64(resp.ReceiveTimeMs),
		}
	}

	return harEntry{
		StartedDateTime: req.GetTime().UTC().Format("2006-01-02T15:04:05.000Z"),
		Time:            timings.Send + timings.Wait + timings.Receive,
		Request:         harReq,
		Response:        harResp,
		Timings:         timings,
	}
}

// assembleHARBody sorts chunks by index and concatenates their data.
func assembleHARBody(chunks []harBodyChunk) []byte {
	if len(chunks) == 0 {
		return nil
	}
	slices.SortFunc(chunks, func(a, b harBodyChunk) int {
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

func harConvertHeaders(headers []*apievents.HTTPHeader) []harNameValue {
	out := make([]harNameValue, 0, len(headers))
	for _, h := range headers {
		out = append(out, harNameValue{Name: h.Name, Value: h.Value})
	}
	return out
}

func harParseQueryString(rawQuery string) []harNameValue {
	vals, _ := url.ParseQuery(rawQuery)
	out := make([]harNameValue, 0, len(vals))
	for k, vs := range vals {
		for _, v := range vs {
			out = append(out, harNameValue{Name: k, Value: v})
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
