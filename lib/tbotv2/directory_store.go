package tbotv2

import (
	"context"
	"github.com/gravitational/trace"
	"os"
	"path"
)

// DirectoryStore implements Writer used by Bot and Destinations as storage
type DirectoryStore struct {
	Dir string
}

func (w *DirectoryStore) path(name string) string {
	return path.Join(w.Dir, name)
}

func (w *DirectoryStore) Write(_ context.Context, name string, data []byte) error {
	// TODO: Sane perms
	return trace.Wrap(os.WriteFile(w.path(name), data, 0666))
}

func (w *DirectoryStore) Read(_ context.Context, name string) ([]byte, error) {
	bytes, err := os.ReadFile(w.path(name))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return bytes, nil
}
