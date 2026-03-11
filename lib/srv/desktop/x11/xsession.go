package x11

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"
)

func GetAvailableXSessions() (map[string]string, error) {
	path, exists := os.LookupEnv("TELEPORT_XSESSIONS_PATH")
	if !exists {
		path = "/usr/share/xsessions"
	}
	entries := make(map[string]string)
	dirEntries, err := os.ReadDir(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, entry := range dirEntries {
		if !strings.HasSuffix(entry.Name(), ".desktop") {
			continue
		}
		file, err := os.Open(filepath.Join(path, entry.Name()))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		scanner := bufio.NewScanner(file)
		var name string
		var exec string
		for scanner.Scan() {
			if s, found := strings.CutPrefix(scanner.Text(), "Name="); found {
				name = s
			} else if s, found := strings.CutPrefix(scanner.Text(), "Exec="); found {
				exec = s
			}
			if name != "" && exec != "" {
				entries[name] = exec
				break
			}
		}
		file.Close()
	}
	return entries, nil
}
