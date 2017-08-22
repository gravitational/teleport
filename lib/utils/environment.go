package utils

import (
	"bufio"
	"os"
	"strings"

	"github.com/gravitational/teleport"

	log "github.com/sirupsen/logrus"
)

// ReadEnvironmentFile will read environment variables from a passed in location.
// Lines that start with "#" or empty lines are ignored. Assignments are in the
// form name=value and no variable expansion occurs.
func ReadEnvironmentFile(filename string) ([]string, error) {
	// open the users environment file. if we don't find a file, move on as
	// having this file for the user is optional.
	file, err := os.Open(filename)
	if err != nil {
		log.Warnf("Unable to open environment file %v: %v, skipping", filename, err)
		return []string{}, nil
	}
	defer file.Close()

	var lineno int
	var envs []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// follow the lead of OpenSSH and don't allow more than 1,000 environment variables
		// https://github.com/openssh/openssh-portable/blob/master/session.c#L873-L874
		lineno = lineno + 1
		if lineno > teleport.MaxEnvironmentFileLines {
			log.Warnf("Too many lines in environment file %v, returning first %v lines", filename, teleport.MaxEnvironmentFileLines)
			return envs, nil
		}

		// empty lines or lines that start with # are ignored
		if line == "" || line[0] == '#' {
			continue
		}

		// split on first =, if not found, log it and continue
		idx := strings.Index(line, "=")
		if idx == -1 {
			log.Debugf("Bad line %v while reading %v: no = separator found", lineno, filename)
			continue
		}

		// split key and value and make sure that key has a name
		key := line[:idx]
		value := line[idx+1:]
		if strings.TrimSpace(key) == "" {
			log.Debugf("Bad line %v while reading %v: key without name", lineno, filename)
			continue
		}

		envs = append(envs, key+"="+value)
	}

	err = scanner.Err()
	if err != nil {
		log.Warnf("Unable to read environment file %v: %v, skipping", filename, err)
		return []string{}, nil
	}

	return envs, nil
}
