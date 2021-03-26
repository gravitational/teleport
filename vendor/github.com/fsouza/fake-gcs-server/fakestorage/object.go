// Copyright 2017 Francisco Souza. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fakestorage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/fsouza/fake-gcs-server/internal/backend"
	"github.com/gorilla/mux"
)

var errInvalidGeneration = errors.New("invalid generation ID")

// Object represents the object that is stored within the fake server.
type Object struct {
	BucketName      string `json:"-"`
	Name            string `json:"name"`
	ContentType     string `json:"contentType"`
	ContentEncoding string `json:"contentEncoding"`
	Content         []byte `json:"-"`
	// Crc32c checksum of Content. calculated by server when it's upload methods are used.
	Crc32c  string            `json:"crc32c,omitempty"`
	Md5Hash string            `json:"md5Hash,omitempty"`
	ACL     []storage.ACLRule `json:"acl,omitempty"`
	// Dates and generation can be manually injected, so you can do assertions on them,
	// or let us fill these fields for you
	Created    time.Time         `json:"created,omitempty"`
	Updated    time.Time         `json:"updated,omitempty"`
	Deleted    time.Time         `json:"deleted,omitempty"`
	Generation int64             `json:"generation,omitempty,string"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

func (o *Object) id() string {
	return o.BucketName + "/" + o.Name
}

type objectList []Object

func (o objectList) Len() int {
	return len(o)
}

func (o objectList) Less(i int, j int) bool {
	return o[i].Name < o[j].Name
}

func (o *objectList) Swap(i int, j int) {
	d := *o
	d[i], d[j] = d[j], d[i]
}

// CreateObject stores the given object internally.
//
// If the bucket within the object doesn't exist, it also creates it. If the
// object already exists, it overrides the object.
func (s *Server) CreateObject(obj Object) {
	_, err := s.createObject(obj)
	if err != nil {
		panic(err)
	}
}

func (s *Server) createObject(obj Object) (Object, error) {
	newObj, err := s.backend.CreateObject(toBackendObjects([]Object{obj})[0])
	if err != nil {
		return Object{}, err
	}

	return fromBackendObjects([]backend.Object{newObj})[0], nil
}

// ListObjects returns a sorted list of objects that match the given criteria,
// or an error if the bucket doesn't exist.
func (s *Server) ListObjects(bucketName, prefix, delimiter string, versions bool) ([]Object, []string, error) {
	backendObjects, err := s.backend.ListObjects(bucketName, versions)
	if err != nil {
		return nil, nil, err
	}
	objects := fromBackendObjects(backendObjects)
	olist := objectList(objects)
	sort.Sort(&olist)
	var respObjects []Object
	prefixes := make(map[string]bool)
	for _, obj := range olist {
		if strings.HasPrefix(obj.Name, prefix) {
			objName := strings.Replace(obj.Name, prefix, "", 1)
			delimPos := strings.Index(objName, delimiter)
			if delimiter != "" && delimPos > -1 {
				prefixes[obj.Name[:len(prefix)+delimPos+1]] = true
			} else {
				respObjects = append(respObjects, obj)
			}
		}
	}
	respPrefixes := make([]string, 0, len(prefixes))
	for p := range prefixes {
		respPrefixes = append(respPrefixes, p)
	}
	sort.Strings(respPrefixes)
	return respObjects, respPrefixes, nil
}

func getCurrentIfZero(date time.Time) time.Time {
	if date.IsZero() {
		return time.Now()
	}
	return date
}

func toBackendObjects(objects []Object) []backend.Object {
	backendObjects := []backend.Object{}
	for _, o := range objects {
		backendObjects = append(backendObjects, backend.Object{
			BucketName:      o.BucketName,
			Name:            o.Name,
			Content:         o.Content,
			ContentType:     o.ContentType,
			ContentEncoding: o.ContentEncoding,
			Crc32c:          o.Crc32c,
			Md5Hash:         o.Md5Hash,
			ACL:             o.ACL,
			Created:         getCurrentIfZero(o.Created).Format(timestampFormat),
			Deleted:         o.Deleted.Format(timestampFormat),
			Updated:         getCurrentIfZero(o.Updated).Format(timestampFormat),
			Generation:      o.Generation,
			Metadata:        o.Metadata,
		})
	}
	return backendObjects
}

func fromBackendObjects(objects []backend.Object) []Object {
	backendObjects := []Object{}
	for _, o := range objects {
		backendObjects = append(backendObjects, Object{
			BucketName:      o.BucketName,
			Name:            o.Name,
			Content:         o.Content,
			ContentType:     o.ContentType,
			ContentEncoding: o.ContentEncoding,
			Crc32c:          o.Crc32c,
			Md5Hash:         o.Md5Hash,
			ACL:             o.ACL,
			Created:         convertTimeWithoutError(o.Created),
			Deleted:         convertTimeWithoutError(o.Deleted),
			Updated:         convertTimeWithoutError(o.Updated),
			Generation:      o.Generation,
			Metadata:        o.Metadata,
		})
	}
	return backendObjects
}

func convertTimeWithoutError(t string) time.Time {
	r, _ := time.Parse(timestampFormat, t)
	return r
}

// GetObject returns the object with the given name in the given bucket, or an
// error if the object doesn't exist.
func (s *Server) GetObject(bucketName, objectName string) (Object, error) {
	backendObj, err := s.backend.GetObject(bucketName, objectName)
	if err != nil {
		return Object{}, err
	}
	obj := fromBackendObjects([]backend.Object{backendObj})[0]
	return obj, nil
}

// GetObjectWithGeneration returns the object with the given name and given
// generation ID in the given bucket, or an error if the object doesn't exist.
//
// If versioning is enabled, archived versions are considered.
func (s *Server) GetObjectWithGeneration(bucketName, objectName string, generation int64) (Object, error) {
	backendObj, err := s.backend.GetObjectWithGeneration(bucketName, objectName, generation)
	if err != nil {
		return Object{}, err
	}
	obj := fromBackendObjects([]backend.Object{backendObj})[0]
	return obj, nil
}

func (s *Server) objectWithGenerationOnValidGeneration(bucketName, objectName, generationStr string) (Object, error) {
	generation, err := strconv.ParseInt(generationStr, 10, 64)
	if err != nil && generationStr != "" {
		return Object{}, errInvalidGeneration
	} else if generation > 0 {
		return s.GetObjectWithGeneration(bucketName, objectName, generation)
	}
	return s.GetObject(bucketName, objectName)
}

func (s *Server) listObjects(w http.ResponseWriter, r *http.Request) {
	bucketName := mux.Vars(r)["bucketName"]
	prefix := r.URL.Query().Get("prefix")
	delimiter := r.URL.Query().Get("delimiter")
	versions := r.URL.Query().Get("versions")

	objs, prefixes, err := s.ListObjects(bucketName, prefix, delimiter, versions == "true")
	encoder := json.NewEncoder(w)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		errResp := newErrorResponse(http.StatusNotFound, http.StatusText(http.StatusNotFound), nil)
		encoder.Encode(errResp)
		return
	}
	encoder.Encode(newListObjectsResponse(objs, prefixes))
}

func (s *Server) getObject(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	if alt := r.URL.Query().Get("alt"); alt == "media" {
		s.downloadObject(w, r)
		return
	}

	encoder := json.NewEncoder(w)
	obj, err := s.objectWithGenerationOnValidGeneration(vars["bucketName"], vars["objectName"], r.FormValue("generation"))
	if err != nil {
		statusCode := http.StatusNotFound
		message := http.StatusText(statusCode)
		if errors.Is(err, errInvalidGeneration) {
			statusCode = http.StatusBadRequest
			message = err.Error()
		}
		errResp := newErrorResponse(statusCode, message, nil)
		w.WriteHeader(statusCode)
		encoder.Encode(errResp)
		return
	}
	w.Header().Set("Accept-Ranges", "bytes")
	encoder.Encode(newObjectResponse(obj))
}

func (s *Server) deleteObject(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	err := s.backend.DeleteObject(vars["bucketName"], vars["objectName"])
	if err != nil {
		errResp := newErrorResponse(http.StatusNotFound, http.StatusText(http.StatusNotFound), nil)
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(errResp)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) listObjectACL(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	obj, err := s.GetObject(vars["bucketName"], vars["objectName"])
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	response := newACLListResponse(obj)
	json.NewEncoder(w).Encode(response)
}

func (s *Server) setObjectACL(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	obj, err := s.GetObject(vars["bucketName"], vars["objectName"])
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	var data struct {
		Entity string
		Role   string
	}

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&data); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	entity := storage.ACLEntity(data.Entity)
	role := storage.ACLRole(data.Role)
	obj.ACL = []storage.ACLRule{{
		Entity: entity,
		Role:   role,
	}}

	s.CreateObject(obj)

	response := newACLListResponse(obj)
	json.NewEncoder(w).Encode(response)
}

func (s *Server) rewriteObject(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	obj, err := s.objectWithGenerationOnValidGeneration(vars["sourceBucket"], vars["sourceObject"], r.FormValue("sourceGeneration"))
	if err != nil {
		statusCode := http.StatusNotFound
		message := http.StatusText(statusCode)
		if errors.Is(err, errInvalidGeneration) {
			statusCode = http.StatusBadRequest
			message = err.Error()
		}
		http.Error(w, message, statusCode)
		return
	}

	var metadata multipartMetadata
	err = json.NewDecoder(r.Body).Decode(&metadata)
	if err != nil && err != io.EOF { // The body is optional
		http.Error(w, "Invalid metadata", http.StatusBadRequest)
		return
	}

	// Only supplied metadata overwrites the new object's metdata
	if len(metadata.Metadata) == 0 {
		metadata.Metadata = obj.Metadata
	}
	if metadata.ContentType == "" {
		metadata.ContentType = obj.ContentType
	}
	if metadata.ContentEncoding == "" {
		metadata.ContentEncoding = obj.ContentEncoding
	}

	dstBucket := vars["destinationBucket"]
	newObject := Object{
		BucketName:      dstBucket,
		Name:            vars["destinationObject"],
		Content:         append([]byte(nil), obj.Content...),
		Crc32c:          obj.Crc32c,
		Md5Hash:         obj.Md5Hash,
		ACL:             obj.ACL,
		ContentType:     metadata.ContentType,
		ContentEncoding: metadata.ContentEncoding,
		Metadata:        metadata.Metadata,
	}

	s.CreateObject(newObject)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newObjectRewriteResponse(newObject))
}

func (s *Server) downloadObject(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	obj, err := s.objectWithGenerationOnValidGeneration(vars["bucketName"], vars["objectName"], r.FormValue("generation"))
	if err != nil {
		statusCode := http.StatusNotFound
		message := http.StatusText(statusCode)
		if errors.Is(err, errInvalidGeneration) {
			statusCode = http.StatusBadRequest
			message = err.Error()
		}
		http.Error(w, message, statusCode)
		return
	}

	status := http.StatusOK
	start, end, content := s.handleRange(obj, r)
	if len(content) != len(obj.Content) {
		status = http.StatusPartialContent
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(obj.Content)))
	}
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Length", strconv.Itoa(len(content)))
	w.Header().Set(contentTypeHeader, obj.ContentType)
	w.Header().Set("X-Goog-Generation", strconv.FormatInt(obj.Generation, 10))
	w.Header().Set("Last-Modified", obj.Updated.Format(http.TimeFormat))
	if obj.ContentEncoding != "" {
		w.Header().Set("Content-Encoding", obj.ContentEncoding)
	}
	w.WriteHeader(status)
	if r.Method == http.MethodGet {
		w.Write(content)
	}
}

func (s *Server) handleRange(obj Object, r *http.Request) (start, end int, content []byte) {
	if reqRange := r.Header.Get("Range"); reqRange != "" {
		parts := strings.SplitN(reqRange, "=", 2)
		if len(parts) == 2 && parts[0] == "bytes" {
			rangeParts := strings.SplitN(parts[1], "-", 2)
			if len(rangeParts) == 2 {
				start, _ = strconv.Atoi(rangeParts[0])
				var err error
				if end, err = strconv.Atoi(rangeParts[1]); err != nil {
					end = len(obj.Content)
				} else {
					end++
				}
				if end > len(obj.Content) {
					end = len(obj.Content)
				}
				return start, end, obj.Content[start:end]
			}
		}
	}
	return 0, 0, obj.Content
}

func (s *Server) patchObject(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	bucketName := vars["bucketName"]
	objectName := vars["objectName"]
	var metadata struct {
		Metadata map[string]string `json:"metadata"`
	}
	err := json.NewDecoder(r.Body).Decode(&metadata)
	if err != nil {
		http.Error(w, "Metadata in the request couldn't decode", http.StatusBadRequest)
		return
	}
	backendObj, err := s.backend.PatchObject(bucketName, objectName, metadata.Metadata)
	if err != nil {
		http.Error(w, "Object not found to be PATCHed", http.StatusNotFound)
		return
	}
	obj := fromBackendObjects([]backend.Object{backendObj})[0]
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newObjectResponse(obj))
}
