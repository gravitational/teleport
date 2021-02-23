package gofakes3

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
)

// routeBase is a http.HandlerFunc that dispatches top level routes for
// GoFakeS3.
//
// URLs are assumed to break down into two common path segments, in the
// following format:
//   /<bucket>/<object>
//
// The operation for most of the core functionality is built around HTTP
// verbs, but outside the core functionality, the clean separation starts
// to degrade, especially around multipart uploads.
//
func (g *GoFakeS3) routeBase(w http.ResponseWriter, r *http.Request) {
	var (
		path   = strings.Trim(r.URL.Path, "/")
		parts  = strings.SplitN(path, "/", 2)
		bucket = parts[0]
		query  = r.URL.Query()
		object = ""
		err    error
	)

	hdr := w.Header()

	id := fmt.Sprintf("%016X", g.nextRequestID())
	hdr.Set("x-amz-id-2", base64.StdEncoding.EncodeToString([]byte(id+id+id+id))) // x-amz-id-2 is 48 bytes of random stuff
	hdr.Set("x-amz-request-id", id)
	hdr.Set("Server", "AmazonS3")

	if len(parts) == 2 {
		object = parts[1]
	}

	if uploadID := UploadID(query.Get("uploadId")); uploadID != "" {
		err = g.routeMultipartUpload(bucket, object, uploadID, w, r)

	} else if _, ok := query["uploads"]; ok {
		err = g.routeMultipartUploadBase(bucket, object, w, r)

	} else if _, ok := query["versioning"]; ok {
		err = g.routeVersioning(bucket, w, r)

	} else if _, ok := query["versions"]; ok {
		err = g.routeVersions(bucket, w, r)

	} else if versionID := versionFromQuery(query["versionId"]); versionID != "" {
		err = g.routeVersion(bucket, object, VersionID(versionID), w, r)

	} else if bucket != "" && object != "" {
		err = g.routeObject(bucket, object, w, r)

	} else if bucket != "" {
		err = g.routeBucket(bucket, w, r)

	} else if r.Method == "GET" {
		err = g.listBuckets(w, r)

	} else {
		http.NotFound(w, r)
		return
	}

	if err != nil {
		g.httpError(w, r, err)
	}
}

// routeObject oandles URLs that contain both a bucket path segment and an
// object path segment.
func (g *GoFakeS3) routeObject(bucket, object string, w http.ResponseWriter, r *http.Request) (err error) {
	switch r.Method {
	case "GET":
		return g.getObject(bucket, object, "", w, r)
	case "HEAD":
		return g.headObject(bucket, object, "", w, r)
	case "PUT":
		return g.createObject(bucket, object, w, r)
	case "DELETE":
		return g.deleteObject(bucket, object, w, r)
	default:
		return ErrMethodNotAllowed
	}
}

// routeBucket handles URLs that contain only a bucket path segment, not an
// object path segment.
func (g *GoFakeS3) routeBucket(bucket string, w http.ResponseWriter, r *http.Request) (err error) {
	switch r.Method {
	case "GET":
		if _, ok := r.URL.Query()["location"]; ok {
			return g.getBucketLocation(bucket, w, r)
		} else {
			return g.listBucket(bucket, w, r)
		}
	case "PUT":
		return g.createBucket(bucket, w, r)
	case "DELETE":
		return g.deleteBucket(bucket, w, r)
	case "HEAD":
		return g.headBucket(bucket, w, r)
	case "POST":
		if _, ok := r.URL.Query()["delete"]; ok {
			return g.deleteMulti(bucket, w, r)
		} else {
			return g.createObjectBrowserUpload(bucket, w, r)
		}
	default:
		return ErrMethodNotAllowed
	}
}

// routeMultipartUploadBase operates on routes that contain '?uploads' in the
// query string. These routes may or may not have a value for bucket or object;
// this is validated and handled in the target handler functions.
func (g *GoFakeS3) routeMultipartUploadBase(bucket, object string, w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case "GET":
		return g.listMultipartUploads(bucket, w, r)
	case "POST":
		return g.initiateMultipartUpload(bucket, object, w, r)
	default:
		return ErrMethodNotAllowed
	}
}

// routeVersioningBase operates on routes that contain '?versioning' in the
// query string. These routes may or may not have a value for bucket; this is
// validated and handled in the target handler functions.
func (g *GoFakeS3) routeVersioning(bucket string, w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case "GET":
		return g.getBucketVersioning(bucket, w, r)
	case "PUT":
		return g.putBucketVersioning(bucket, w, r)
	default:
		return ErrMethodNotAllowed
	}
}

// routeVersions operates on routes that contain '?versions' in the query string.
func (g *GoFakeS3) routeVersions(bucket string, w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case "GET":
		return g.listBucketVersions(bucket, w, r)
	default:
		return ErrMethodNotAllowed
	}
}

// routeVersion operates on routes that contain '?versionId=<id>' in the
// query string.
func (g *GoFakeS3) routeVersion(bucket, object string, versionID VersionID, w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case "GET":
		return g.getObject(bucket, object, versionID, w, r)
	case "HEAD":
		return g.headObject(bucket, object, versionID, w, r)
	case "DELETE":
		return g.deleteObjectVersion(bucket, object, versionID, w, r)
	default:
		return ErrMethodNotAllowed
	}
}

// routeMultipartUpload operates on routes that contain '?uploadId=<id>' in the
// query string.
func (g *GoFakeS3) routeMultipartUpload(bucket, object string, uploadID UploadID, w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case "GET":
		return g.listMultipartUploadParts(bucket, object, uploadID, w, r)
	case "PUT":
		return g.putMultipartUploadPart(bucket, object, uploadID, w, r)
	case "DELETE":
		return g.abortMultipartUpload(bucket, object, uploadID, w, r)
	case "POST":
		return g.completeMultipartUpload(bucket, object, uploadID, w, r)
	default:
		return ErrMethodNotAllowed
	}
}

func versionFromQuery(qv []string) string {
	// The versionId subresource may be the string 'null'; this has been
	// observed coming in via Boto. The S3 documentation for the "DELETE
	// object" endpoint describes a 'null' version explicitly, but we don't
	// want backend implementers to have to special-case this string, so
	// let's hide it in here:
	if len(qv) > 0 && qv[0] != "" && qv[0] != "null" {
		return qv[0]
	}
	return ""
}
