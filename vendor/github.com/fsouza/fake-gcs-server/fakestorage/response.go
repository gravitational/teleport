// Copyright 2017 Francisco Souza. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fakestorage

import (
	"net/http"

	"github.com/fsouza/fake-gcs-server/internal/backend"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/storage/v1"
)

const timestampFormat = "2006-01-02T15:04:05.999999Z07:00"

type listResponse struct {
	Kind     string        `json:"kind"`
	Items    []interface{} `json:"items"`
	Prefixes []string      `json:"prefixes,omitempty"`
}

func newListBucketsResponse(buckets []backend.Bucket) listResponse {
	resp := listResponse{
		Kind:  "storage#buckets",
		Items: make([]interface{}, len(buckets)),
	}
	for i, bucket := range buckets {
		resp.Items[i] = newBucketResponse(bucket)
	}
	return resp
}

type bucketResponse struct {
	Kind        string            `json:"kind"`
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Versioning  *bucketVersioning `json:"versioning,omitempty"`
	TimeCreated string            `json:"timeCreated,omitempty"`
}

type bucketVersioning struct {
	Enabled bool `json:"enabled,omitempty"`
}

func newBucketResponse(bucket backend.Bucket) bucketResponse {
	return bucketResponse{
		Kind:        "storage#bucket",
		ID:          bucket.Name,
		Name:        bucket.Name,
		Versioning:  &bucketVersioning{bucket.VersioningEnabled},
		TimeCreated: bucket.TimeCreated.Format(timestampFormat),
	}
}

func newListObjectsResponse(objs []Object, prefixes []string) listResponse {
	resp := listResponse{
		Kind:     "storage#objects",
		Items:    make([]interface{}, len(objs)),
		Prefixes: prefixes,
	}
	for i, obj := range objs {
		resp.Items[i] = newObjectResponse(obj)
	}
	return resp
}

type objectResponse struct {
	Kind            string                         `json:"kind"`
	Name            string                         `json:"name"`
	ID              string                         `json:"id"`
	Bucket          string                         `json:"bucket"`
	Size            int64                          `json:"size,string"`
	ContentType     string                         `json:"contentType,omitempty"`
	ContentEncoding string                         `json:"contentEncoding,omitempty"`
	Crc32c          string                         `json:"crc32c,omitempty"`
	ACL             []*storage.ObjectAccessControl `json:"acl,omitempty"`
	Md5Hash         string                         `json:"md5Hash,omitempty"`
	TimeCreated     string                         `json:"timeCreated,omitempty"`
	TimeDeleted     string                         `json:"timeDeleted,omitempty"`
	Updated         string                         `json:"updated,omitempty"`
	Generation      int64                          `json:"generation,string"`
	Metadata        map[string]string              `json:"metadata,omitempty"`
}

func newObjectResponse(obj Object) objectResponse {
	acl := getAccessControlsListFromObject(obj)

	return objectResponse{
		Kind:            "storage#object",
		ID:              obj.id(),
		Bucket:          obj.BucketName,
		Name:            obj.Name,
		Size:            int64(len(obj.Content)),
		ContentType:     obj.ContentType,
		ContentEncoding: obj.ContentEncoding,
		Crc32c:          obj.Crc32c,
		Md5Hash:         obj.Md5Hash,
		ACL:             acl,
		Metadata:        obj.Metadata,
		TimeCreated:     obj.Created.Format(timestampFormat),
		TimeDeleted:     obj.Deleted.Format(timestampFormat),
		Updated:         obj.Updated.Format(timestampFormat),
		Generation:      obj.Generation,
	}
}

type aclListResponse struct {
	*storage.ObjectAccessControls
}

func newACLListResponse(obj Object) aclListResponse {
	if len(obj.ACL) == 0 {
		return aclListResponse{}
	}

	aclItems := getAccessControlsListFromObject(obj)

	return aclListResponse{
		&storage.ObjectAccessControls{
			ServerResponse: googleapi.ServerResponse{
				HTTPStatusCode: http.StatusOK,
			},
			Items: aclItems,
		},
	}
}

func getAccessControlsListFromObject(obj Object) []*storage.ObjectAccessControl {
	aclItems := make([]*storage.ObjectAccessControl, len(obj.ACL))
	for idx, aclRule := range obj.ACL {
		aclItems[idx] = &storage.ObjectAccessControl{
			Bucket: obj.BucketName,
			Entity: string(aclRule.Entity),
			Object: obj.Name,
			Role:   string(aclRule.Role),
		}
	}
	return aclItems
}

type rewriteResponse struct {
	Kind                string         `json:"kind"`
	TotalBytesRewritten int64          `json:"totalBytesRewritten,string"`
	ObjectSize          int64          `json:"objectSize,string"`
	Done                bool           `json:"done"`
	RewriteToken        string         `json:"rewriteToken"`
	Resource            objectResponse `json:"resource"`
}

func newObjectRewriteResponse(obj Object) rewriteResponse {
	return rewriteResponse{
		Kind:                "storage#rewriteResponse",
		TotalBytesRewritten: int64(len(obj.Content)),
		ObjectSize:          int64(len(obj.Content)),
		Done:                true,
		RewriteToken:        "",
		Resource:            newObjectResponse(obj),
	}
}

type errorResponse struct {
	Error httpError `json:"error"`
}

type httpError struct {
	Code    int        `json:"code"`
	Message string     `json:"message"`
	Errors  []apiError `json:"errors"`
}

type apiError struct {
	Domain  string `json:"domain"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

func newErrorResponse(code int, message string, errs []apiError) errorResponse {
	return errorResponse{
		Error: httpError{
			Code:    code,
			Message: message,
			Errors:  errs,
		},
	}
}
