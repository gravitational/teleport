package ztpki

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/hiyosi/hawk"
)

func NewClientWithHawk(server string, credentials hawk.Credential) (*ClientWithResponses, error) {
	return NewClientWithResponses(server, WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		hawkClient := &hawk.Client{
			Credential: &credentials,
			Option: &hawk.Option{
				TimeStamp: time.Now().Unix(),
				Nonce:     hex.EncodeToString(binary.NativeEndian.AppendUint64(nil, rand.Uint64())),
			},
		}

		if req.Body != nil && req.Body != http.NoBody {
			if req.GetBody == nil {
				return errors.New("missing GetBody for request with body")
			}

			body, err := req.GetBody()
			if err != nil {
				return err
			}
			defer body.Close()

			bodyData, err := io.ReadAll(body)
			if err != nil {
				return err
			}

			hawkClient.Option.Payload = string(bodyData)
			hawkClient.Option.ContentType = req.Header.Get("Content-Type")
		}

		authorization, err := hawkClient.Header(req.Method, req.URL.String())
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", authorization)

		return nil
	}))
}
