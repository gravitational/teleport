package ttyterminal

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/gravitational/trace"
	"github.com/pkoukk/tiktoken-go"

	"github.com/gravitational/teleport/lib/defaults"
)

var (
	tke     *tiktoken.Tiktoken
	tkeOnce sync.Once
	tkeErr  error
)

var openAIBaseCDNURL = "https://openaipublic.blob.core.windows.net/encodings/"

const (
	bpeFileName      = "o200k_base.tiktoken"
	o200kBaseHash    = "446a9538cb6c348e3516120d7c08b09f57c36495e2acfffe59a5bf8b0cfb1a2d"
	bpeFileDirEnvVar = "TELEPORT_TIKTOKEN_BPE_DIR"
)

// initTokenizer initializes the tiktoken encoder for token counting.
// It uses sync.Once to ensure it's only initialized once.
func initTokenizer() (*tiktoken.Tiktoken, error) {
	tkeOnce.Do(func() {
		loader := newBPELoader()
		tiktoken.SetBpeLoader(loader)

		tke, tkeErr = tiktoken.GetEncoding("o200k_base")
	})

	return tke, tkeErr
}

func newBPELoader() *bpeLoader {
	return &bpeLoader{
		cdnURL:       openAIBaseCDNURL,
		fileDir:      os.Getenv(bpeFileDirEnvVar),
		expectedHash: o200kBaseHash,
	}
}

// CountTokens counts the number of tokens in a string using the o200k_base encoding.
// If the tokenizer fails to initialize, it returns 0.
func CountTokens(text string) int {
	tk, err := initTokenizer()
	if err != nil {
		return 0
	}

	tokens := tk.Encode(text, nil, nil)
	return len(tokens)
}

type bpeLoader struct {
	cdnURL       string
	fileDir      string
	expectedHash string
	// httpClient is used for testing purposes to allow injection of a custom HTTP client.
	// If nil, the default HTTP client will be used.
	httpClient *http.Client
}

// LoadTiktokenBpe is called by tiktoken to load BPE data. This implementation
// checks to see if the BPE file exists locally, if not, it will fetch it from
// the OpenAI CDN (same as the default loader).
func (b *bpeLoader) LoadTiktokenBpe(name string) (map[string]int, error) {
	var (
		bpe []byte
		err error
	)
	switch {
	case b.fileDir != "":
		bpe, err = b.loadBpeFromFile()
	default:
		bpe, err = b.loadBpeFromCDN()
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := b.verifyHash(bpe); err != nil {
		return nil, trace.Wrap(err)
	}

	bpeRanks := make(map[string]int)
	for line := range bytes.Lines(bpe) {
		if len(line) == 0 {
			continue
		}

		parts := bytes.Split(line, []byte(" "))
		token := make([]byte, base64.StdEncoding.DecodedLen(len(parts[0])))
		n, err := base64.StdEncoding.Decode(token, parts[0])
		if err != nil {
			return nil, trace.Wrap(err)
		}
		token = token[:n]

		if len(parts) != 2 {
			return nil, trace.BadParameter("invalid BPE line: %q", line)
		}

		rank, err := strconv.Atoi(string(bytes.TrimSuffix(parts[1], []byte("\n"))))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		bpeRanks[string(token)] = rank
	}

	return bpeRanks, nil
}

func (b *bpeLoader) loadBpeFromFile() ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(b.fileDir, bpeFileName))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return data, nil
}

func (b *bpeLoader) loadBpeFromCDN() ([]byte, error) {
	url, err := url.JoinPath(b.cdnURL, bpeFileName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	httpClient := b.httpClient
	if httpClient == nil {
		httpClient, err = defaults.HTTPClient(defaults.UseProxyFromEnvironment())
		if err != nil {
			return nil, trace.Wrap(err, "failed to create HTTP client for fetching BPE from CDN")
		}
	}

	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, trace.BadParameter("failed to fetch BPE from CDN: %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return data, nil
}

func (b *bpeLoader) verifyHash(data []byte) error {
	hash := sha256.Sum256(data)
	actualHash := hex.EncodeToString(hash[:])

	if actualHash != b.expectedHash {
		return trace.BadParameter("BPE file hash mismatch: expected %s, got %s", b.expectedHash, actualHash)
	}

	return nil
}
