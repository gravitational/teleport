/*
Copyright 2019 Gravitational, Inc.

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

package events

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"io"
	"io/ioutil"
	"strings"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// ValidateServerMetadata checks that event server ID of the event
// if present, matches the passed server ID and namespace has proper syntax
func ValidateServerMetadata(event AuditEvent, serverID string) error {
	getter, ok := event.(ServerMetadataGetter)
	if !ok {
		return nil
	}
	if getter.GetServerID() != serverID {
		return trace.BadParameter("server %q can't emit event with server ID %q", serverID, getter.GetServerID())
	}
	if ns := getter.GetServerNamespace(); ns != "" && !types.IsValidNamespace(ns) {
		return trace.BadParameter("invalid namespace %q", ns)
	}
	return nil
}

// UpdateEventFields updates passed event fields with additional information
// common for all event types such as unique IDs, timestamps, codes, etc.
//
// This method is a "final stop" for various audit log implementations for
// updating event fields before it gets persisted in the backend.
func UpdateEventFields(event Event, fields EventFields, clock clockwork.Clock, uid utils.UID) (err error) {
	additionalFields := make(map[string]interface{})
	if fields.GetType() == "" {
		additionalFields[EventType] = event.Name
	}
	if fields.GetID() == "" {
		additionalFields[EventID] = uid.New()
	}
	if fields.GetTimestamp().IsZero() {
		additionalFields[EventTime] = clock.Now().UTC().Round(time.Second)
	}
	if event.Code != "" {
		additionalFields[EventCode] = event.Code
	}
	for k, v := range additionalFields {
		fields[k] = v
	}
	return nil
}

// ValidateEvent checks the the fields within an event match the passed in
// expected values.
func ValidateEvent(f EventFields, serverID string) error {
	if f.HasField(SessionServerID) && f.GetString(SessionServerID) != serverID {
		return trace.BadParameter("server ID %v not valid", f.GetString(SessionServerID))
	}
	if f.HasField(EventNamespace) && !types.IsValidNamespace(f.GetString(EventNamespace)) {
		return trace.BadParameter("invalid namespace %v", f.GetString(EventNamespace))
	}

	return nil
}

// ValidateArchive validates namespace and serverID fields within all events
// in the archive.
func ValidateArchive(reader io.Reader, serverID string) error {
	tarball := tar.NewReader(reader)

	for {
		header, err := tarball.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return trace.Wrap(err)
		}

		// Skip over any file in the archive that doesn't contain session events.
		if !strings.HasSuffix(header.Name, eventsSuffix) {
			_, err = io.Copy(ioutil.Discard, tarball)
			if err != nil {
				return trace.Wrap(err)
			}
			continue
		}

		zip, err := gzip.NewReader(tarball)
		if err != nil {
			return trace.Wrap(err)
		}
		defer zip.Close()

		scanner := bufio.NewScanner(zip)
		for scanner.Scan() {
			var f EventFields
			err := utils.FastUnmarshal(scanner.Bytes(), &f)
			if err != nil {
				return trace.Wrap(err)
			}
			err = ValidateEvent(f, serverID)
			if err != nil {
				return trace.Wrap(err)
			}
		}
		if err := scanner.Err(); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}
