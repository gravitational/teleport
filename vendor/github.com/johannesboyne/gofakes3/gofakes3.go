package gofakes3

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// GoFakeS3 implements HTTP handlers for processing S3 requests and returning
// S3 responses.
//
// Logic is delegated to other components, like Backend or uploader.
type GoFakeS3 struct {
	requestID uint64

	storage   Backend
	versioned VersionedBackend

	timeSource              TimeSource
	timeSkew                time.Duration
	metadataSizeLimit       int
	integrityCheck          bool
	failOnUnimplementedPage bool
	hostBucket              bool
	uploader                *uploader
	log                     Logger
}

// New creates a new GoFakeS3 using the supplied Backend. Backends are pluggable.
// Several Backend implementations ship with GoFakeS3, which can be found in the
// gofakes3/backends package.
func New(backend Backend, options ...Option) *GoFakeS3 {
	s3 := &GoFakeS3{
		storage:           backend,
		timeSkew:          DefaultSkewLimit,
		metadataSizeLimit: DefaultMetadataSizeLimit,
		integrityCheck:    true,
		uploader:          newUploader(),
		requestID:         0,
	}

	// versioned MUST be set before options as one of the options disables it:
	s3.versioned, _ = backend.(VersionedBackend)

	for _, opt := range options {
		opt(s3)
	}
	if s3.log == nil {
		s3.log = DiscardLog()
	}
	if s3.timeSource == nil {
		s3.timeSource = DefaultTimeSource()
	}

	return s3
}

func (g *GoFakeS3) nextRequestID() uint64 {
	return atomic.AddUint64(&g.requestID, 1)
}

// Create the AWS S3 API
func (g *GoFakeS3) Server() http.Handler {
	var handler http.Handler = &withCORS{r: http.HandlerFunc(g.routeBase), log: g.log}

	if g.timeSkew != 0 {
		handler = g.timeSkewMiddleware(handler)
	}

	if g.hostBucket {
		handler = g.hostBucketMiddleware(handler)
	}

	return handler
}

func (g *GoFakeS3) timeSkewMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, rq *http.Request) {
		timeHdr := rq.Header.Get("x-amz-date")

		if timeHdr != "" {
			rqTime, _ := time.Parse("20060102T150405Z", timeHdr)
			at := g.timeSource.Now()
			skew := at.Sub(rqTime)

			if skew < -g.timeSkew || skew > g.timeSkew {
				g.httpError(w, rq, requestTimeTooSkewed(at, g.timeSkew))
				return
			}
		}

		handler.ServeHTTP(w, rq)
	})
}

// hostBucketMiddleware forces the server to use VirtualHost-style bucket URLs:
// https://docs.aws.amazon.com/AmazonS3/latest/dev/UsingBucket.html
func (g *GoFakeS3) hostBucketMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, rq *http.Request) {
		parts := strings.SplitN(rq.Host, ".", 2)
		bucket := parts[0]

		p := rq.URL.Path
		rq.URL.Path = "/" + bucket
		if p != "/" {
			rq.URL.Path += p
		}
		g.log.Print(LogInfo, p, "=>", rq.URL)

		handler.ServeHTTP(w, rq)
	})
}

func (g *GoFakeS3) httpError(w http.ResponseWriter, r *http.Request, err error) {
	resp := ensureErrorResponse(err, "") // FIXME: request id
	if resp.ErrorCode() == ErrInternal {
		g.log.Print(LogErr, err)
	}

	w.WriteHeader(resp.ErrorCode().Status())

	if r.Method != http.MethodHead {
		if err := g.xmlEncoder(w).Encode(resp); err != nil {
			g.log.Print(LogErr, err)
			return
		}
	}
}

func (g *GoFakeS3) listBuckets(w http.ResponseWriter, r *http.Request) error {
	buckets, err := g.storage.ListBuckets()
	if err != nil {
		return err
	}

	s := &Storage{
		Xmlns:   "http://s3.amazonaws.com/doc/2006-03-01/",
		Buckets: buckets,
		Owner: &UserInfo{
			ID:          "fe7272ea58be830e56fe1663b10fafef",
			DisplayName: "GoFakeS3",
		},
	}

	return g.xmlEncoder(w).Encode(s)
}

// S3 has two versions of this API, both of which are close to identical. We manage that
// jank in here so the Backend doesn't have to with the following tricks:
//
// - Hiding the NextMarker inside the ContinuationToken for V2 calls
// - Masking the Owner in the response for V2 calls
//
// The wrapping response objects are slightly different too, but the list of
// objects is pretty much the same.
//
// - https://docs.aws.amazon.com/AmazonS3/latest/API/RESTBucketGET.html
// - https://docs.aws.amazon.com/AmazonS3/latest/API/v2-RESTBucketGET.html
//
func (g *GoFakeS3) listBucket(bucketName string, w http.ResponseWriter, r *http.Request) error {
	g.log.Print(LogInfo, "LIST BUCKET")

	q := r.URL.Query()
	prefix := prefixFromQuery(q)
	page, err := listBucketPageFromQuery(q)
	if err != nil {
		return err
	}

	isVersion2 := q.Get("list-type") == "2"

	g.log.Print(LogInfo, "bucketname:", bucketName)
	g.log.Print(LogInfo, "prefix    :", prefix)
	g.log.Print(LogInfo, "page      :", fmt.Sprintf("%+v", page))

	objects, err := g.storage.ListBucket(bucketName, &prefix, page)

	if err != nil {
		if err == ErrInternalPageNotImplemented && !g.failOnUnimplementedPage {
			// We have observed (though not yet confirmed) that simple clients
			// tend to work fine if you simply ignore pagination, so the
			// default if this is not implemented is to retry without it. If
			// you care about this performance impact for some weird reason,
			// you'll need to handle it yourself.
			objects, err = g.storage.ListBucket(bucketName, &prefix, ListBucketPage{})
			if err != nil {
				return err
			}

		} else if err == ErrInternalPageNotImplemented && g.failOnUnimplementedPage {
			return ErrNotImplemented
		} else {
			return err
		}
	}

	base := ListBucketResultBase{
		Xmlns:          "http://s3.amazonaws.com/doc/2006-03-01/",
		Name:           bucketName,
		CommonPrefixes: objects.CommonPrefixes,
		Contents:       objects.Contents,
		IsTruncated:    objects.IsTruncated,
		Delimiter:      prefix.Delimiter,
		Prefix:         prefix.Prefix,
		MaxKeys:        page.MaxKeys,
	}

	if !isVersion2 {
		var result = &ListBucketResult{
			ListBucketResultBase: base,
			Marker:               page.Marker,
		}
		if base.Delimiter != "" {
			// From the S3 docs: "This element is returned only if you specify
			// a delimiter request parameter." Dunno why. This hack has been moved
			// into GoFakeS3 to spare backend implementers the trouble.
			result.NextMarker = objects.NextMarker
		}
		return g.xmlEncoder(w).Encode(result)

	} else {
		var result = &ListBucketResultV2{
			ListBucketResultBase: base,
			KeyCount:             int64(len(objects.CommonPrefixes) + len(objects.Contents)),
			StartAfter:           q.Get("start-after"),
			ContinuationToken:    q.Get("continuation-token"),
		}
		if objects.NextMarker != "" {
			// We are just cheating with these continuation tokens; they're just the NextMarker
			// from v1 in disguise! That may change at any time and should not be relied upon
			// though.
			result.NextContinuationToken = base64.URLEncoding.EncodeToString([]byte(objects.NextMarker))
		}

		// On the topic of "fetch-owner", the AWS docs say, in typically vague style:
		// "If you want the owner information in the response, you can specify
		// this parameter with the value set to true."
		//
		// What does the bare word 'true' mean when we're talking about a query
		// string parameter, which can only be a string? Does it mean the word
		// 'true'? Does it mean 'any truthy string'? Does it mean only the key
		// needs to be present (i.e. '?fetch-owner'), which we are assuming
		// for now? This is why you need proper technical writers.
		//
		// Probably need to hit up the s3assumer at some point, but until then, here's
		// another FIXME!
		if _, ok := q["fetch-owner"]; !ok {
			for _, v := range result.Contents {
				v.Owner = nil
			}
		}

		return g.xmlEncoder(w).Encode(result)
	}
}

func (g *GoFakeS3) getBucketLocation(bucketName string, w http.ResponseWriter, r *http.Request) error {
	g.log.Print(LogInfo, "GET BUCKET LOCATION")

	result := GetBucketLocation{
		Xmlns:              "http://s3.amazonaws.com/doc/2006-03-01/",
		LocationConstraint: "",
	}

	return g.xmlEncoder(w).Encode(result)
}

func (g *GoFakeS3) listBucketVersions(bucketName string, w http.ResponseWriter, r *http.Request) error {
	if g.versioned == nil {
		return ErrNotImplemented
	}

	q := r.URL.Query()
	prefix := prefixFromQuery(q)
	page, err := listBucketVersionsPageFromQuery(q)
	if err != nil {
		return err
	}

	// S300004:
	if page.HasVersionIDMarker {
		if page.VersionIDMarker == "" {
			return ErrorInvalidArgument("version-id-marker", "", "A version-id marker cannot be empty.")
		} else if !page.HasKeyMarker {
			return ErrorInvalidArgument("version-id-marker", "", "A version-id marker cannot be specified without a key marker.")
		}

	} else if page.HasKeyMarker && page.KeyMarker == "" {
		// S300004: S3 ignores everything if you pass an empty key marker so
		// let's hide that bit of ugliness from Backend.
		page = ListBucketVersionsPage{}
	}

	bucket, err := g.versioned.ListBucketVersions(bucketName, &prefix, &page)
	if err != nil {
		return err
	}

	for _, ver := range bucket.Versions {
		// S300005: S3 returns the _string_ 'null' for the version ID if the
		// bucket has never had versioning enabled. GoFakeS3 backend
		// implementers should be able to simply return the empty string;
		// GoFakeS3 itself should handle this particular bit of jank once and
		// once only.
		if ver.GetVersionID() == "" {
			ver.setVersionID("null")
		}
	}

	return g.xmlEncoder(w).Encode(bucket)
}

// CreateBucket creates a new S3 bucket in the BoltDB storage.
func (g *GoFakeS3) createBucket(bucket string, w http.ResponseWriter, r *http.Request) error {
	g.log.Print(LogInfo, "CREATE BUCKET:", bucket)

	if err := ValidateBucketName(bucket); err != nil {
		return err
	}
	if err := g.storage.CreateBucket(bucket); err != nil {
		return err
	}

	w.Header().Set("Location", "/"+bucket)
	w.Write([]byte{})
	return nil
}

// DeleteBucket deletes the bucket in the underlying backend, if and only if it
// contains no items.
func (g *GoFakeS3) deleteBucket(bucket string, w http.ResponseWriter, r *http.Request) error {
	g.log.Print(LogInfo, "DELETE BUCKET:", bucket)
	if err := g.storage.DeleteBucket(bucket); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// HeadBucket checks whether a bucket exists.
func (g *GoFakeS3) headBucket(bucket string, w http.ResponseWriter, r *http.Request) error {
	g.log.Print(LogInfo, "HEAD BUCKET", bucket)
	g.log.Print(LogInfo, "bucketname:", bucket)

	if err := g.ensureBucketExists(bucket); err != nil {
		return err
	}

	w.Write([]byte{})
	return nil
}

// GetObject retrievs a bucket object.
func (g *GoFakeS3) getObject(
	bucket, object string,
	versionID VersionID,
	w http.ResponseWriter,
	r *http.Request,
) error {

	g.log.Print(LogInfo, "GET OBJECT")
	g.log.Print(LogInfo, "Bucket:", bucket)
	g.log.Print(LogInfo, "└── Object:", object)

	rnge, err := parseRangeHeader(r.Header.Get("Range"))
	if err != nil {
		return err
	}

	var obj *Object

	{ // get object from backend
		if versionID == "" {
			obj, err = g.storage.GetObject(bucket, object, rnge)
			if err != nil {
				return err
			}
		} else {
			if g.versioned == nil {
				return ErrNotImplemented
			}
			obj, err = g.versioned.GetObjectVersion(bucket, object, versionID, rnge)
			if err != nil {
				return err
			}
		}
	}

	if obj == nil {
		g.log.Print(LogErr, "unexpected nil object for key", bucket, object)
		return ErrInternal
	}
	defer obj.Contents.Close()

	if err := g.writeGetOrHeadObjectResponse(obj, w, r); err != nil {
		return err
	}

	// Writes Content-Length, and Content-Range if applicable:
	obj.Range.writeHeader(obj.Size, w)

	if _, err := io.Copy(w, obj.Contents); err != nil {
		return err
	}

	return nil
}

// writeGetOrHeadObjectResponse contains shared logic for constructing headers for
// a HEAD and a GET request for a /bucket/object URL.
func (g *GoFakeS3) writeGetOrHeadObjectResponse(obj *Object, w http.ResponseWriter, r *http.Request) error {
	// "If the current version of the object is a delete marker, Amazon S3
	// behaves as if the object was deleted and includes x-amz-delete-marker:
	// true in the response."
	if obj.IsDeleteMarker {
		w.Header().Set("x-amz-version-id", string(obj.VersionID))
		w.Header().Set("x-amz-delete-marker", "true")
		return KeyNotFound(obj.Name)
	}

	for mk, mv := range obj.Metadata {
		w.Header().Set(mk, mv)
	}
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("ETag", `"`+hex.EncodeToString(obj.Hash)+`"`)

	if obj.VersionID != "" {
		w.Header().Set("x-amz-version-id", string(obj.VersionID))
	}
	return nil
}

// headObject retrieves only meta information of an object and not the whole.
func (g *GoFakeS3) headObject(
	bucket, object string,
	versionID VersionID,
	w http.ResponseWriter,
	r *http.Request,
) error {

	g.log.Print(LogInfo, "HEAD OBJECT")
	g.log.Print(LogInfo, "Bucket:", bucket)
	g.log.Print(LogInfo, "└── Object:", object)

	obj, err := g.storage.HeadObject(bucket, object)
	if err != nil {
		return err
	}
	if obj == nil {
		g.log.Print(LogErr, "unexpected nil object for key", bucket, object)
		return ErrInternal
	}
	defer obj.Contents.Close()

	if err := g.writeGetOrHeadObjectResponse(obj, w, r); err != nil {
		return err
	}

	w.Header().Set("Content-Length", fmt.Sprintf("%d", obj.Size))

	return nil
}

// createObjectBrowserUpload allows objects to be created from a multipart upload initiated
// by a browser form.
func (g *GoFakeS3) createObjectBrowserUpload(bucket string, w http.ResponseWriter, r *http.Request) error {
	g.log.Print(LogInfo, "CREATE OBJECT THROUGH BROWSER UPLOAD")

	const _24MB = (1 << 20) * 24 // maximum amount of memory before temp files are used
	if err := r.ParseMultipartForm(_24MB); nil != err {
		return ErrMalformedPOSTRequest
	}

	keyValues := r.MultipartForm.Value["key"]
	if len(keyValues) != 1 {
		return ErrIncorrectNumberOfFilesInPostRequest
	}
	key := keyValues[0]

	g.log.Print(LogInfo, "(BUC)", bucket)
	g.log.Print(LogInfo, "(KEY)", key)

	fileValues := r.MultipartForm.File["file"]
	if len(fileValues) != 1 {
		return ErrIncorrectNumberOfFilesInPostRequest
	}
	fileHeader := fileValues[0]

	infile, err := fileHeader.Open()
	if err != nil {
		return err
	}
	defer infile.Close()

	meta, err := metadataHeaders(r.MultipartForm.Value, g.timeSource.Now(), g.metadataSizeLimit)
	if err != nil {
		return err
	}

	if len(key) > KeySizeLimit {
		return ResourceError(ErrKeyTooLong, key)
	}

	// FIXME: how does Content-MD5 get sent when using the browser? does it?
	rdr, err := newHashingReader(infile, "")
	if err != nil {
		return err
	}

	result, err := g.storage.PutObject(bucket, key, meta, rdr, fileHeader.Size)
	if err != nil {
		return err
	}
	if result.VersionID != "" {
		w.Header().Set("x-amz-version-id", string(result.VersionID))
	}

	w.Header().Set("ETag", `"`+hex.EncodeToString(rdr.Sum(nil))+`"`)
	return nil
}

// CreateObject creates a new S3 object.
func (g *GoFakeS3) createObject(bucket, object string, w http.ResponseWriter, r *http.Request) (err error) {
	g.log.Print(LogInfo, "CREATE OBJECT:", bucket, object)

	meta, err := metadataHeaders(r.Header, g.timeSource.Now(), g.metadataSizeLimit)
	if err != nil {
		return err
	}

	if _, ok := meta["X-Amz-Copy-Source"]; ok {
		return g.copyObject(bucket, object, meta, w, r)
	}

	contentLength := r.Header.Get("Content-Length")
	if contentLength == "" {
		return ErrMissingContentLength
	}

	size, err := strconv.ParseInt(contentLength, 10, 64)
	if err != nil || size < 0 {
		w.WriteHeader(http.StatusBadRequest) // XXX: no code for this, according to s3tests
		return nil
	}

	if len(object) > KeySizeLimit {
		return ResourceError(ErrKeyTooLong, object)
	}

	var md5Base64 string
	if g.integrityCheck {
		md5Base64 = r.Header.Get("Content-MD5")

		if _, ok := r.Header[textproto.CanonicalMIMEHeaderKey("Content-MD5")]; ok && md5Base64 == "" {
			return ErrInvalidDigest // Satisfies s3tests
		}
	}

	var reader io.Reader

	if sha, ok := meta["X-Amz-Content-Sha256"]; ok && sha == "STREAMING-AWS4-HMAC-SHA256-PAYLOAD" {
		reader = newChunkedReader(r.Body)
		size, err = strconv.ParseInt(meta["X-Amz-Decoded-Content-Length"], 10, 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest) // XXX: no code for this, according to s3tests
			return nil
		}
	} else {
		reader = r.Body
	}

	// hashingReader is still needed to get the ETag even if integrityCheck
	// is set to false:
	rdr, err := newHashingReader(reader, md5Base64)
	defer r.Body.Close()
	if err != nil {
		return err
	}

	result, err := g.storage.PutObject(bucket, object, meta, rdr, size)
	if err != nil {
		return err
	}

	if result.VersionID != "" {
		g.log.Print(LogInfo, "CREATED VERSION:", bucket, object, result.VersionID)
		w.Header().Set("x-amz-version-id", string(result.VersionID))
	}
	w.Header().Set("ETag", `"`+hex.EncodeToString(rdr.Sum(nil))+`"`)

	return nil
}

// CopyObject copies an existing S3 object
func (g *GoFakeS3) copyObject(bucket, object string, meta map[string]string, w http.ResponseWriter, r *http.Request) (err error) {
	source := meta["X-Amz-Copy-Source"]
	g.log.Print(LogInfo, "└── COPY:", source)

	if len(object) > KeySizeLimit {
		return ResourceError(ErrKeyTooLong, object)
	}

	// XXX No support for versionId subresource
	parts := strings.SplitN(strings.TrimPrefix(source, "/"), "/", 2)
	srcBucket := parts[0]
	srcKey := strings.SplitN(parts[1], "?", 2)[0]

	srcObj, err := g.storage.GetObject(srcBucket, srcKey, nil)
	if err != nil {
		return err
	}

	if srcObj == nil {
		g.log.Print(LogErr, "unexpected nil object for key", bucket, object)
		return ErrInternal
	}
	defer srcObj.Contents.Close()

	// XXX No support for delete marker
	// "If the current version of the object is a delete marker, Amazon S3
	// behaves as if the object was deleted."

	// merge metadata, ACL is not preserved
	for k, v := range srcObj.Metadata {
		if _, found := meta[k]; !found && k != "X-Amz-Acl" {
			meta[k] = v
		}
	}

	result, err := g.storage.PutObject(bucket, object, meta, srcObj.Contents, srcObj.Size)
	if err != nil {
		return err
	}

	if srcObj.VersionID != "" {
		w.Header().Set("x-amz-copy-source-version-id", string(srcObj.VersionID))
	}
	if result.VersionID != "" {
		g.log.Print(LogInfo, "CREATED VERSION:", bucket, object, result.VersionID)
		w.Header().Set("x-amz-version-id", string(result.VersionID))
	}

	return g.xmlEncoder(w).Encode(CopyObjectResult{
		ETag:         `"` + hex.EncodeToString(srcObj.Hash) + `"`,
		LastModified: NewContentTime(g.timeSource.Now()),
	})
}

func (g *GoFakeS3) deleteObject(bucket, object string, w http.ResponseWriter, r *http.Request) error {
	g.log.Print(LogInfo, "DELETE:", bucket, object)
	result, err := g.storage.DeleteObject(bucket, object)
	if err != nil {
		return err
	}

	if result.IsDeleteMarker {
		w.Header().Set("x-amz-delete-marker", "true")
	} else {
		w.Header().Set("x-amz-delete-marker", "false")
	}

	if result.VersionID != "" {
		w.Header().Set("x-amz-version-id", string(result.VersionID))
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (g *GoFakeS3) deleteObjectVersion(bucket, object string, version VersionID, w http.ResponseWriter, r *http.Request) error {
	if g.versioned == nil {
		return ErrNotImplemented
	}

	g.log.Print(LogInfo, "DELETE VERSION:", bucket, object, version)
	result, err := g.versioned.DeleteObjectVersion(bucket, object, version)
	if err != nil {
		return err
	}
	g.log.Print(LogInfo, "DELETED VERSION:", bucket, object, version)

	if result.IsDeleteMarker {
		w.Header().Set("x-amz-delete-marker", "true")
	} else {
		w.Header().Set("x-amz-delete-marker", "false")
	}

	if result.VersionID != "" {
		w.Header().Set("x-amz-version-id", string(result.VersionID))
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// deleteMulti deletes multiple S3 objects from the bucket.
// https://docs.aws.amazon.com/AmazonS3/latest/API/multiobjectdeleteapi.html
func (g *GoFakeS3) deleteMulti(bucket string, w http.ResponseWriter, r *http.Request) error {
	g.log.Print(LogInfo, "delete multi", bucket)

	var in DeleteRequest

	defer r.Body.Close()
	dc := xml.NewDecoder(r.Body)
	if err := dc.Decode(&in); err != nil {
		return ErrorMessage(ErrMalformedXML, err.Error())
	}

	keys := make([]string, len(in.Objects))
	for i, o := range in.Objects {
		keys[i] = o.Key
	}

	out, err := g.storage.DeleteMulti(bucket, keys...)
	if err != nil {
		return err
	}

	if in.Quiet {
		out.Deleted = nil
	}

	return g.xmlEncoder(w).Encode(out)
}

func (g *GoFakeS3) initiateMultipartUpload(bucket, object string, w http.ResponseWriter, r *http.Request) error {
	g.log.Print(LogInfo, "initiate multipart upload", bucket, object)

	meta, err := metadataHeaders(r.Header, g.timeSource.Now(), g.metadataSizeLimit)
	if err != nil {
		return err
	}
	if err := g.ensureBucketExists(bucket); err != nil {
		return err
	}

	upload := g.uploader.Begin(bucket, object, meta, g.timeSource.Now())
	out := InitiateMultipartUpload{
		UploadID: upload.ID,
		Bucket:   bucket,
		Key:      object,
	}
	return g.xmlEncoder(w).Encode(out)
}

// From the docs:
//	A part number uniquely identifies a part and also defines its position
// 	within the object being created. If you upload a new part using the same
// 	part number that was used with a previous part, the previously uploaded part
// 	is overwritten. Each part must be at least 5 MB in size, except the last
// 	part. There is no size limit on the last part of your multipart upload.
//
func (g *GoFakeS3) putMultipartUploadPart(bucket, object string, uploadID UploadID, w http.ResponseWriter, r *http.Request) error {
	g.log.Print(LogInfo, "put multipart upload", bucket, object, uploadID)

	partNumber, err := strconv.ParseInt(r.URL.Query().Get("partNumber"), 10, 0)
	if err != nil || partNumber <= 0 || partNumber > MaxUploadPartNumber {
		return ErrInvalidPart
	}

	size, err := strconv.ParseInt(r.Header.Get("Content-Length"), 10, 64)
	if err != nil || size <= 0 {
		return ErrMissingContentLength
	}

	upload, err := g.uploader.Get(bucket, object, uploadID)
	if err != nil {
		// FIXME: What happens with S3 when you abort a multipart upload while
		// part uploads are still in progress? In this case, we will retain the
		// reference to the part even though another request goroutine may
		// delete it; it will be available for GC when this function finishes.
		return err
	}

	defer r.Body.Close()
	var rdr io.Reader = r.Body

	if g.integrityCheck {
		md5Base64 := r.Header.Get("Content-MD5")
		if _, ok := r.Header[textproto.CanonicalMIMEHeaderKey("Content-MD5")]; ok && md5Base64 == "" {
			return ErrInvalidDigest // Satisfies s3tests
		}

		if md5Base64 != "" {
			var err error
			rdr, err = newHashingReader(rdr, md5Base64)
			if err != nil {
				return err
			}
		}
	}

	body, err := ReadAll(rdr, size)
	if err != nil {
		return err
	}

	if int64(len(body)) != r.ContentLength {
		return ErrIncompleteBody
	}

	etag, err := upload.AddPart(int(partNumber), g.timeSource.Now(), body)
	if err != nil {
		return err
	}

	w.Header().Add("ETag", etag)
	return nil
}

func (g *GoFakeS3) abortMultipartUpload(bucket, object string, uploadID UploadID, w http.ResponseWriter, r *http.Request) error {
	g.log.Print(LogInfo, "abort multipart upload", bucket, object, uploadID)
	if _, err := g.uploader.Complete(bucket, object, uploadID); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (g *GoFakeS3) completeMultipartUpload(bucket, object string, uploadID UploadID, w http.ResponseWriter, r *http.Request) error {
	g.log.Print(LogInfo, "complete multipart upload", bucket, object, uploadID)

	var in CompleteMultipartUploadRequest
	if err := g.xmlDecodeBody(r.Body, &in); err != nil {
		return err
	}

	upload, err := g.uploader.Complete(bucket, object, uploadID)
	if err != nil {
		return err
	}

	fileBody, etag, err := upload.Reassemble(&in)
	if err != nil {
		return err
	}

	result, err := g.storage.PutObject(bucket, object, upload.Meta, bytes.NewReader(fileBody), int64(len(fileBody)))
	if err != nil {
		return err
	}
	if result.VersionID != "" {
		w.Header().Set("x-amz-version-id", string(result.VersionID))
	}

	return g.xmlEncoder(w).Encode(&CompleteMultipartUploadResult{
		ETag:   etag,
		Bucket: bucket,
		Key:    object,
	})
}

func (g *GoFakeS3) listMultipartUploads(bucket string, w http.ResponseWriter, r *http.Request) error {
	query := r.URL.Query()
	prefix := prefixFromQuery(query)
	marker := uploadListMarkerFromQuery(query)

	maxUploads, err := parseClampedInt(query.Get("max-uploads"), DefaultMaxUploads, 0, MaxUploadsLimit)
	if err != nil {
		return ErrInvalidURI
	}
	if maxUploads == 0 {
		maxUploads = DefaultMaxUploads
	}

	out, err := g.uploader.List(bucket, marker, prefix, maxUploads)
	if err != nil {
		return err
	}

	return g.xmlEncoder(w).Encode(out)
}

func (g *GoFakeS3) listMultipartUploadParts(bucket, object string, uploadID UploadID, w http.ResponseWriter, r *http.Request) error {
	query := r.URL.Query()

	marker, err := parseClampedInt(query.Get("part-number-marker"), 0, 0, math.MaxInt64)
	if err != nil {
		return ErrInvalidURI
	}

	maxParts, err := parseClampedInt(query.Get("max-parts"), DefaultMaxUploadParts, 0, MaxUploadPartsLimit)
	if err != nil {
		return ErrInvalidURI
	}

	out, err := g.uploader.ListParts(bucket, object, uploadID, int(marker), maxParts)
	if err != nil {
		return err
	}

	return g.xmlEncoder(w).Encode(out)
}

func (g *GoFakeS3) getBucketVersioning(bucket string, w http.ResponseWriter, r *http.Request) error {
	var config VersioningConfiguration

	if g.versioned != nil {
		var err error
		config, err = g.versioned.VersioningConfiguration(bucket)
		if err != nil {
			return err
		}
	}

	return g.xmlEncoder(w).Encode(config)
}

func (g *GoFakeS3) putBucketVersioning(bucket string, w http.ResponseWriter, r *http.Request) error {
	var in VersioningConfiguration
	if err := g.xmlDecodeBody(r.Body, &in); err != nil {
		return err
	}

	if g.versioned == nil {
		if in.MFADelete == MFADeleteEnabled || in.Status == VersioningEnabled {
			// We only need to respond that this is not implemented if there's an
			// attempt to enable it. If we receive a request to disable it, or an
			// empty request, that matches the current state and has no effect so
			// we can accept it.
			return ErrNotImplemented
		} else {
			return nil
		}
	}

	g.log.Print(LogInfo, "PUT VERSIONING:", in.Status)
	return g.versioned.SetVersioningConfiguration(bucket, in)
}

func (g *GoFakeS3) ensureBucketExists(bucket string) error {
	exists, err := g.storage.BucketExists(bucket)
	if err != nil {
		return err
	}
	if !exists {
		return ResourceError(ErrNoSuchBucket, bucket)
	}
	return nil
}

func (g *GoFakeS3) xmlEncoder(w http.ResponseWriter) *xml.Encoder {
	w.Write([]byte(xml.Header))
	w.Header().Set("Content-Type", "application/xml")

	xe := xml.NewEncoder(w)
	xe.Indent("", "  ")
	return xe
}

func (g *GoFakeS3) xmlDecodeBody(rdr io.ReadCloser, into interface{}) error {
	body, err := ioutil.ReadAll(rdr)
	defer rdr.Close()
	if err != nil {
		return err
	}

	if err := xml.Unmarshal(body, into); err != nil {
		return ErrorMessage(ErrMalformedXML, err.Error())
	}

	return nil
}

func formatHeaderTime(t time.Time) string {
	// https://github.com/aws/aws-sdk-go/issues/1937 - FIXED
	// https://github.com/aws/aws-sdk-go-v2/issues/178 - Still open
	// .Format("Mon, 2 Jan 2006 15:04:05 MST")

	tc := t.In(time.UTC)
	return tc.Format("Mon, 02 Jan 2006 15:04:05") + " GMT"
}

func metadataSize(meta map[string]string) int {
	total := 0
	for k, v := range meta {
		total += len(k) + len(v)
	}
	return total
}

func metadataHeaders(headers map[string][]string, at time.Time, sizeLimit int) (map[string]string, error) {
	meta := make(map[string]string)
	for hk, hv := range headers {
		if strings.HasPrefix(hk, "X-Amz-") {
			meta[hk] = hv[0]
		}
	}
	meta["Last-Modified"] = formatHeaderTime(at)

	if sizeLimit > 0 && metadataSize(meta) > sizeLimit {
		return meta, ErrMetadataTooLarge
	}

	return meta, nil
}

func listBucketPageFromQuery(query url.Values) (page ListBucketPage, rerr error) {
	maxKeys, err := parseClampedInt(query.Get("max-keys"), DefaultMaxBucketKeys, 0, MaxBucketKeys)
	if err != nil {
		return page, err
	}

	page.MaxKeys = maxKeys

	if _, page.HasMarker = query["marker"]; page.HasMarker {
		// List Objects V1 uses marker only:
		page.Marker = query.Get("marker")

	} else if _, page.HasMarker = query["continuation-token"]; page.HasMarker {
		// List Objects V2 uses continuation-token preferentially, or
		// start-after if continuation-token is missing. continuation-token is
		// an opaque value that looks like this: 1ueGcxLPRx1Tr/XYExHnhbYLgveDs2J/wm36Hy4vbOwM=.
		// This just looks like base64 junk so we just cheat and base64 encode
		// the next marker and hide it in a continuation-token.
		tok, err := base64.URLEncoding.DecodeString(query.Get("continuation-token"))
		if err != nil {
			// FIXME: log
			return page, ErrInvalidToken // FIXME: confirm for sure what AWS does here
		}
		page.Marker = string(tok)

	} else if _, page.HasMarker = query["start-after"]; page.HasMarker {
		// List Objects V2 uses start-after if continuation-token is missing:
		page.Marker = query.Get("start-after")
	}

	return page, nil
}

func listBucketVersionsPageFromQuery(query url.Values) (page ListBucketVersionsPage, rerr error) {
	maxKeys, err := parseClampedInt(query.Get("max-keys"), DefaultMaxBucketVersionKeys, 0, MaxBucketVersionKeys)
	if err != nil {
		return page, err
	}

	page.MaxKeys = maxKeys
	page.KeyMarker = query.Get("key-marker")
	page.VersionIDMarker = VersionID(query.Get("version-id-marker"))
	_, page.HasKeyMarker = query["key-marker"]
	_, page.HasVersionIDMarker = query["version-id-marker"]

	return page, nil
}
