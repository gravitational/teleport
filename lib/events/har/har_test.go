package har

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
)

func event(t time.Time) apievents.Metadata { return apievents.Metadata{Time: t} }

func TestBuilder_RequestResponse(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	b := NewBuilder()
	b.Add(&apievents.AppSessionHTTPRequest{
		Metadata: event(now), RequestId: "r1", Method: "POST",
		Url: "https://api.anthropic.com/v1/messages?beta=1", HttpVersion: "HTTP/1.1",
		RawQuery: "beta=1",
		Headers:  []*apievents.HTTPHeader{{Name: "Content-Type", Value: "application/json"}},
	})
	b.Add(&apievents.AppSessionHTTPRequestBodyChunk{RequestId: "r1", ChunkIndex: 1, Data: []byte(`rld"}`)})
	b.Add(&apievents.AppSessionHTTPRequestBodyChunk{RequestId: "r1", ChunkIndex: 0, Data: []byte(`{"q":"hello wo`)})
	b.Add(&apievents.AppSessionHTTPResponse{
		RequestId: "r1", StatusCode: 200, HttpVersion: "HTTP/1.1",
		Headers:    []*apievents.HTTPHeader{{Name: "Content-Type", Value: "application/json"}},
		WaitTimeMs: 12,
	})
	b.Add(&apievents.AppSessionHTTPResponseBodyChunk{RequestId: "r1", ChunkIndex: 0, Data: []byte(`{"ok":true}`)})

	var buf bytes.Buffer
	require.NoError(t, b.Encode(&buf))

	var root struct {
		Log struct {
			Version string `json:"version"`
			Entries []struct {
				Request struct {
					Method   string `json:"method"`
					PostData struct {
						Text string `json:"text"`
					} `json:"postData"`
					QueryString []struct{ Name, Value string } `json:"queryString"`
				} `json:"request"`
				Response struct {
					Status  int `json:"status"`
					Content struct {
						Text string `json:"text"`
					} `json:"content"`
				} `json:"response"`
			} `json:"entries"`
		} `json:"log"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &root))
	require.Equal(t, "1.2", root.Log.Version)
	require.Len(t, root.Log.Entries, 1)
	e := root.Log.Entries[0]
	require.Equal(t, "POST", e.Request.Method)
	require.Equal(t, `{"q":"hello world"}`, e.Request.PostData.Text)
	require.Equal(t, 200, e.Response.Status)
	require.Equal(t, `{"ok":true}`, e.Response.Content.Text)
	require.Equal(t, []struct{ Name, Value string }{{"beta", "1"}}, e.Request.QueryString)
}

func TestBuilder_BinaryBodyBase64(t *testing.T) {
	t.Parallel()
	b := NewBuilder()
	b.Add(&apievents.AppSessionHTTPRequest{
		RequestId: "r1", Method: "POST", Url: "https://x/y",
		Headers: []*apievents.HTTPHeader{{Name: "Content-Type", Value: "application/octet-stream"}},
	})
	b.Add(&apievents.AppSessionHTTPRequestBodyChunk{RequestId: "r1", ChunkIndex: 0, Data: []byte{0x00, 0xff}})
	var buf bytes.Buffer
	require.NoError(t, b.Encode(&buf))
	require.Contains(t, buf.String(), `"encoding": "base64"`)
}

func TestBuilder_RequestWithoutResponse(t *testing.T) {
	t.Parallel()
	b := NewBuilder()
	b.Add(&apievents.AppSessionHTTPRequest{RequestId: "r1", Method: "GET", Url: "https://x/y"})
	var buf bytes.Buffer
	require.NoError(t, b.Encode(&buf))
	require.Contains(t, buf.String(), `"status": 0`)
}

func TestParseAndIndex(t *testing.T) {
	t.Parallel()
	b := NewBuilder()
	b.Add(&apievents.AppSessionHTTPRequest{
		RequestId: "r1", Method: "POST", Url: "https://x/a",
		Headers: []*apievents.HTTPHeader{{Name: "Content-Type", Value: "application/json"}},
	})
	b.Add(&apievents.AppSessionHTTPRequestBodyChunk{RequestId: "r1", ChunkIndex: 0, Data: []byte(`{"k":1}`)})
	b.Add(&apievents.AppSessionHTTPResponse{
		RequestId: "r1", StatusCode: 201,
		Headers: []*apievents.HTTPHeader{{Name: "Content-Type", Value: "application/json"}},
	})
	b.Add(&apievents.AppSessionHTTPResponseBodyChunk{RequestId: "r1", ChunkIndex: 0, Data: []byte(`{"ok":true}`)})
	b.Add(&apievents.AppSessionHTTPRequest{RequestId: "r2", Method: "GET", Url: "https://x/b"})

	var buf bytes.Buffer
	require.NoError(t, b.Encode(&buf))

	root, err := Parse(buf.Bytes())
	require.NoError(t, err)

	idx := root.Index()
	require.Len(t, idx, 2)

	require.Equal(t, 0, idx[0].Index)
	require.Equal(t, "POST", idx[0].Method)
	require.Equal(t, "https://x/a", idx[0].URL)
	require.Equal(t, 201, idx[0].StatusCode)
	require.Equal(t, "application/json", idx[0].ResponseMIMEType)
	require.Equal(t, len(`{"ok":true}`), idx[0].ResponseBodySize)
	require.Equal(t, len(`{"k":1}`), idx[0].RequestBodySize)

	// r2 has a request but no response.
	require.Equal(t, 1, idx[1].Index)
	require.Equal(t, "GET", idx[1].Method)
	require.Equal(t, 0, idx[1].StatusCode)

	// Entries are retrievable by index for on-demand loading.
	require.Len(t, root.Log.Entries, 2)
}
