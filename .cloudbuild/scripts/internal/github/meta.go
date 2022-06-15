package github

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

const (
	metadataURL = "https://api.github.com/meta"
)

// Metadata contains information about the github networking environment,
// enclosing host keys for git/ssh access
type Metadata struct {
	SSHKeyFingerPrints map[string]string `json:"ssh_key_fingerprints"`
	SSHKeys            []string          `json:"ssh_keys"`
}

func GetMetadata(ctx context.Context) (*Metadata, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataURL, nil)
	if err != nil {
		return nil, trace.Wrap(err, "failed creating metadata request")
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, trace.Wrap(err, "failed issuing metadata request")
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, trace.Errorf("Metadata request failed %d", response.StatusCode)
	}

	var meta Metadata
	err = json.NewDecoder(response.Body).Decode(&meta)
	if err != nil {
		return nil, trace.Wrap(err, "failed parsing metadata")
	}

	return &meta, nil
}

func (meta *Metadata) HostKeys() ([]ssh.PublicKey, error) {
	keys := make([]ssh.PublicKey, 0, len(meta.SSHKeys))
	for _, text := range meta.SSHKeys {
		key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(text))
		if err != nil {
			return nil, trace.Wrap(err, "failed parsing host key")
		}
		keys = append(keys, key)
	}
	return keys, nil
}
