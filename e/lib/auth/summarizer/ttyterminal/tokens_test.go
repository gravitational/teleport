package ttyterminal

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

const validBPEContent = `Aw== 0
Ag== 1
AQ== 2
BA== 3
Bg== 4`

var validBPEContentHash = func() string {
	hash := sha256.Sum256([]byte(validBPEContent))
	return hex.EncodeToString(hash[:])
}()

func TestBpeLoader_LoadTiktokenBpe(t *testing.T) {
	validateBPEResult := func(t *testing.T, result map[string]int) {
		require.Len(t, result, 5)
		require.Equal(t, 0, result["\x03"])
		require.Equal(t, 1, result["\x02"])
		require.Equal(t, 2, result["\x01"])
		require.Equal(t, 3, result["\x04"])
		require.Equal(t, 4, result["\x06"])
	}

	writeValidBPE := func(t *testing.T, path string) {
		err := os.WriteFile(path, []byte(validBPEContent), 0o644)
		require.NoError(t, err)
	}

	t.Run("loads from file when exists", func(t *testing.T) {
		tempDir := t.TempDir()
		writeValidBPE(t, filepath.Join(tempDir, "o200k_base.tiktoken"))

		loader, server, cdnCalled := setupLoader(tempDir, http.StatusOK)
		loader.expectedHash = validBPEContentHash
		defer server.Close()

		result, err := loader.LoadTiktokenBpe("o200k_base")
		require.NoError(t, err)
		require.False(t, *cdnCalled)
		validateBPEResult(t, result)
	})

	t.Run("falls back to CDN when file not found", func(t *testing.T) {
		loader, server, cdnCalled := setupLoader("", http.StatusOK)
		loader.expectedHash = validBPEContentHash
		defer server.Close()

		result, err := loader.LoadTiktokenBpe("o200k_base")
		require.NoError(t, err)
		require.True(t, *cdnCalled)
		validateBPEResult(t, result)
	})

	t.Run("returns error for invalid file content", func(t *testing.T) {
		tempDir := t.TempDir()
		err := os.WriteFile(filepath.Join(tempDir, "o200k_base.tiktoken"), []byte("invalid base64"), 0o644)
		require.NoError(t, err)

		loader, server, cdnCalled := setupLoader(tempDir, http.StatusOK)
		loader.expectedHash = "1234567890abcdef"
		defer server.Close()

		_, err = loader.LoadTiktokenBpe("o200k_base")
		require.Error(t, err)
		require.ErrorContains(t, err, "hash mismatch")
		require.False(t, *cdnCalled)
	})

	t.Run("returns error when hash verification fails", func(t *testing.T) {
		tempDir := t.TempDir()
		writeValidBPE(t, filepath.Join(tempDir, "o200k_base.tiktoken"))

		loader, server, cdnCalled := setupLoader(tempDir, http.StatusOK)
		loader.expectedHash = "incorrecthash1234567890abcdef1234567890abcdef1234567890abcdef1234"
		defer server.Close()

		_, err := loader.LoadTiktokenBpe("o200k_base")
		require.Error(t, err)
		require.True(t, trace.IsBadParameter(err))
		require.ErrorContains(t, err, "BPE file hash mismatch")
		require.False(t, *cdnCalled)
	})

	t.Run("returns error when CDN returns non-200", func(t *testing.T) {
		loader, server, cdnCalled := setupLoader("", http.StatusNotFound)
		defer server.Close()

		_, err := loader.LoadTiktokenBpe("o200k_base")
		require.Error(t, err)
		require.True(t, trace.IsBadParameter(err))
		require.ErrorContains(t, err, "failed to fetch BPE from CDN")
		require.True(t, *cdnCalled)
	})

	t.Run("returns error when CDN request fails", func(t *testing.T) {
		cdnCalled := false
		server := httptest.NewServer(nil)
		defer server.Close()

		loader := &bpeLoader{
			cdnURL:  server.URL + "/",
			fileDir: "",
			httpClient: &http.Client{
				Transport: &errorTransport{
					err:    fmt.Errorf("network error"),
					called: &cdnCalled,
				},
			},
		}

		_, err := loader.LoadTiktokenBpe("o200k_base")
		require.Error(t, err)
		require.ErrorContains(t, err, "network error")
		require.True(t, cdnCalled)
	})
}

func TestBpeLoader_LoadBpeFromFile(t *testing.T) {
	t.Run("reads from default dir", func(t *testing.T) {
		tempDir := t.TempDir()
		content := []byte("test content")
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, "o200k_base.tiktoken"), content, 0o644))

		loader := &bpeLoader{fileDir: tempDir}
		data, err := loader.loadBpeFromFile()
		require.NoError(t, err)
		require.Equal(t, content, data)
	})

	t.Run("reads from env dir when set", func(t *testing.T) {
		tempDir := t.TempDir()
		envDir := filepath.Join(tempDir, "env")
		require.NoError(t, os.MkdirAll(envDir, 0o755))

		content := []byte("env content")
		require.NoError(t, os.WriteFile(filepath.Join(envDir, "o200k_base.tiktoken"), content, 0o644))
		t.Setenv("TELEPORT_TIKTOKEN_BPE_DIR", envDir)

		loader := newBPELoader()
		data, err := loader.loadBpeFromFile()
		require.NoError(t, err)
		require.Equal(t, content, data)
	})

	t.Run("returns error when file doesn't exist", func(t *testing.T) {
		tempDir := t.TempDir()
		loader := &bpeLoader{fileDir: tempDir}

		_, err := loader.loadBpeFromFile()
		require.Error(t, err)
		require.Contains(t, err.Error(), "no such file or directory")
	})
}

func TestBpeLoader_LoadBpeFromCDN(t *testing.T) {
	t.Run("successful download", func(t *testing.T) {
		content := []byte("cdn content")
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(content)
		}))
		defer server.Close()

		loader := &bpeLoader{
			cdnURL:     server.URL + "/",
			httpClient: server.Client(),
		}

		data, err := loader.loadBpeFromCDN()
		require.NoError(t, err)
		require.Equal(t, content, data)
	})

	t.Run("handles non-200 response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		loader := &bpeLoader{
			cdnURL:     server.URL + "/",
			httpClient: server.Client(),
		}

		_, err := loader.loadBpeFromCDN()
		require.Error(t, err)
		require.True(t, trace.IsBadParameter(err))
		require.ErrorContains(t, err, "failed to fetch BPE from CDN")
	})
}

func TestBpeLoader_VerifyHash(t *testing.T) {
	t.Run("verifies correct hash", func(t *testing.T) {
		data := []byte("test data")
		hash := sha256.Sum256(data)
		expectedHash := hex.EncodeToString(hash[:])

		loader := &bpeLoader{
			expectedHash: expectedHash,
		}

		err := loader.verifyHash(data)
		require.NoError(t, err)
	})

	t.Run("fails on incorrect hash", func(t *testing.T) {
		data := []byte("test data")

		loader := &bpeLoader{
			expectedHash: "incorrecthash",
		}

		err := loader.verifyHash(data)
		require.Error(t, err)
		require.True(t, trace.IsBadParameter(err))
		require.ErrorContains(t, err, "BPE file hash mismatch")
	})

	t.Run("verifies actual o200k_base hash", func(t *testing.T) {
		loader := &bpeLoader{
			expectedHash: o200kBaseHash,
		}

		if testing.Short() {
			t.Skip("Skipping real file download test in short mode")
		}

		resp, err := http.Get("https://openaipublic.blob.core.windows.net/encodings/o200k_base.tiktoken")
		if err != nil {
			t.Skip("Could not fetch real BPE file, skipping test")
		}
		defer resp.Body.Close()

		data, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		err = loader.verifyHash(data)
		require.NoError(t, err)
	})
}

func setupLoader(tempDir string, cdnStatus int) (*bpeLoader, *httptest.Server, *bool) {
	cdnCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cdnCalled = true

		w.WriteHeader(cdnStatus)
		if cdnStatus == http.StatusOK {
			w.Write([]byte(validBPEContent))
		}
	}))

	loader := &bpeLoader{
		cdnURL:     server.URL + "/",
		fileDir:    tempDir,
		httpClient: server.Client(),
	}

	return loader, server, &cdnCalled
}

type errorTransport struct {
	err    error
	called *bool
}

func (e *errorTransport) RoundTrip(*http.Request) (*http.Response, error) {
	if e.called != nil {
		*e.called = true
	}
	return nil, e.err
}
