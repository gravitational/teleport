/*
Copyright 2023 Gravitational, Inc.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/gravitational/trace"
)

var ignoredEventTypes = map[string]struct{}{
	"device": {},
}

func main() {
	if len(os.Args) != 2 {
		log.Fatal(trace.BadParameter("the executable must be called with a single argument: the path of the events/api.go file."))
	}
	filePath := os.Args[1]
	mapping := iterateOverFile(filePath, getMappingEntry)
	output, err := dumpMapping(mapping)
	if err != nil {
		log.Fatal(err)
	}
	// TODO: run fmt before outputting
	fmt.Println(string(output))
}

var reString = `\s*([^ ]+)Event\s+=\s+"([^ ]+)"`
var re = regexp.MustCompile(reString)

func getMappingEntry(line []byte) (string, string) {
	if !re.Match(line) {
		return "", ""
	}

	matches := re.FindSubmatch(line)
	if len(matches) != 3 {
		return "", ""
	}
	eventType := string(matches[2])
	messageName := string(matches[1])
	if _, ok := ignoredEventTypes[eventType]; ok {
		return "", ""
	}
	return eventType, messageName
}

func iterateOverFile(path string, fn func([]byte) (string, string)) map[string]string {
	file, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	mapping := map[string]string{}
	reader := bufio.NewReader(file)

	for {
		line, _, err := reader.ReadLine()

		if err == io.EOF {
			break
		}
		key, val := fn(line)
		if key != "" {
			mapping[key] = val
		}
	}
	return mapping
}

const mainTemplate = `// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package eventschema

// This file is generated, DO NOT EDIT

var eventTypes = []string {
{{- range $eventType, $_ := .Mapping }}
	{{ $eventType | quote }},
{{- end }}
}
`

func dumpMapping(mapping map[string]string) ([]byte, error) {
	t := template.New("*")
	t = t.Funcs(sprig.FuncMap())
	t = template.Must(t.Parse(mainTemplate))

	input := struct {
		Mapping map[string]string
	}{
		mapping,
	}

	buf := &bytes.Buffer{}
	err := t.Execute(buf, input)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return buf.Bytes(), nil
}
