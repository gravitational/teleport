package endtoend

import (
	"bytes"
	"html/template"
	"os"
	"strings"

	"github.com/gravitational/trace"
)

type configData struct {
	DataDir    string
	StorageDir string
}

func newConfiguration(name string, yaml string) (string, error) {
	testDir, err := os.MkdirTemp("", strings.Join([]string{name, "data"}, "-"))
	if err != nil {
		return "", trace.Wrap(err)
	}
	//defer os.RemoveAll(dir) // clean up
	storageDir, err := os.MkdirTemp("", strings.Join([]string{name, "storage"}, "-"))
	if err != nil {
		return "", trace.Wrap(err)
	}
	//defer os.RemoveAll(dir) // clean up

	t, err := template.New(name).Parse(yaml)
	if err != nil {
		return "", trace.Wrap(err)
	}

	var buffer bytes.Buffer
	err = t.Execute(&buffer, &configData{
		DataDir:    dataDir,
		StorageDir: storageDir,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}

	return buffer.String(), nil
}

type teleport struct {
}

func newTeleport(config string) error {
	return nil
}
