package gofakes3

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/johannesboyne/gofakes3/internal/goskipiter"
	"github.com/ryszard/goskiplist/skiplist"
)

var add1 = new(big.Int).SetInt64(1)

/*
bucketUploads maintains a map of buckets to the list of multipart uploads
for that bucket.

A skiplist that maps object keys to upload ids is also maintained to
support the ListMultipartUploads operation.

From the docs:
	In the response, the uploads are sorted by key. If your application has
	initiated more than one multipart upload using the same object key,
	then uploads in the response are first sorted by key. Additionally,
	uploads are sorted in ascending order within each key by the upload
	initiation time.

It's ambiguous whether "sorted by key" means "sorted by the upload ID"
or "sorted by the object key". It's also ambiguous whether the docs mean
the sorting applies only within an individual page of results, or to the
whole result across all paginations. This is supported somewhat, though
not unambiguously, by the documentation for "key-marker" and
"upload-id-marker":

	key-marker: Together with upload-id-marker, this parameter specifies the
	multipart upload after which listing should begin.

	If upload-id-marker is not specified, only the keys lexicographically
	greater than the specified key-marker will be included in the list.

	If upload-id-marker is specified, any multipart uploads for a key equal to
	the key-marker might also be included, provided those multipart uploads
	have upload IDs lexicographically greater than the specified
	upload-id-marker.

	upload-id-marker: Together with key-marker, specifies the multipart upload
	after which listing should begin. If key-marker is not specified, the
	upload-id-marker parameter is ignored.

This implementation assumes "sorted by key" means "sorted by the object
key" and that the sorting applies across the full pagination set.

The SkipList provides O(log n) performance, but the slices inside are
linear-time. This should provide an acceptable trade-off for simplicity;
on my 2013-era i7 machine, a simple linear search for the last element
in a 100,000 element array of 80-ish byte strings takes barely 1ms.
*/
type bucketUploads struct {
	// uploads should be protected by the coarse lock in uploader:
	uploads map[UploadID]*multipartUpload

	// objectIndex provides sorted traversal of the bucket uploads.
	//
	// The keys in this skiplist are the object keys, the values are the slice
	// of *multipartUpload structs associated with that key. The skiplist
	// satisfies the map ordering constraint, the slice satisfies the upload
	// initiation time constraint.
	objectIndex *skiplist.SkipList // effectively map[ObjectKey][]*multipartUpload
}

func newBucketUploads() *bucketUploads {
	return &bucketUploads{
		uploads:     map[UploadID]*multipartUpload{},
		objectIndex: skiplist.NewStringMap(),
	}
}

// add assumes uploader.mu is acquired
func (bu *bucketUploads) add(mpu *multipartUpload) {
	bu.uploads[mpu.ID] = mpu

	uploads, ok := bu.objectIndex.Get(mpu.Object)
	if !ok {
		uploads = []*multipartUpload{mpu}
	} else {
		uploads = append(uploads.([]*multipartUpload), mpu)
	}
	bu.objectIndex.Set(mpu.Object, uploads)
}

// remove assumes uploader.mu is acquired
func (bu *bucketUploads) remove(uploadID UploadID) {
	upload := bu.uploads[uploadID]
	delete(bu.uploads, uploadID)

	var uploads []*multipartUpload
	{
		upv, ok := bu.objectIndex.Get(upload.Object)
		if !ok || upv == nil {
			return
		}
		uploads = upv.([]*multipartUpload)
	}

	var found = -1
	var v *multipartUpload
	for found, v = range uploads {
		if v.ID == uploadID {
			break
		}
	}

	if found >= 0 {
		uploads = append(uploads[:found], uploads[found+1:]...) // delete the found index
	}

	if len(uploads) == 0 {
		bu.objectIndex.Delete(upload.Object)
	} else {
		bu.objectIndex.Set(upload.Object, uploads)
	}
}

// uploader manages multipart uploads.
//
// Multipart upload support has the following rather severe limitations (which
// will hopefully be addressed in the future):
//
//	- uploads do not interface with the Backend, so they do not
// 	  currently persist across reboots
//
//	- upload parts are held in memory, so if you want to upload something huge
//	  in multiple parts (which is pretty much exactly what you'd want multipart
//	  uploads for), you'll need to make sure your memory is also sufficiently
//	  huge!
//
// At this stage, the current thinking would be to add a second optional
// Backend interface that allows persistent operations on multipart upload
// data, and if a Backend does not implement it, this limited in-memory
// behaviour can be the fallback. If that can be made to work, it would provide
// good convenience for Backend implementers if their use case did not require
// persistent multipart upload handling, or it could be satisfied by this
// naive implementation.
//
type uploader struct {
	// uploadIDs use a big.Int to allow unbounded IDs (not that you'd be
	// expected to ever generate 4.2 billion of these but who are we to judge?)
	uploadID *big.Int

	buckets map[string]*bucketUploads
	mu      sync.Mutex
}

func newUploader() *uploader {
	return &uploader{
		buckets:  make(map[string]*bucketUploads),
		uploadID: new(big.Int),
	}
}

func (u *uploader) Begin(bucket, object string, meta map[string]string, initiated time.Time) *multipartUpload {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.uploadID.Add(u.uploadID, add1)

	mpu := &multipartUpload{
		ID:        UploadID(u.uploadID.String()),
		Bucket:    bucket,
		Object:    object,
		Meta:      meta,
		Initiated: initiated,
	}

	// FIXME: make sure the uploader responds to DeleteBucket
	bucketUploads := u.buckets[bucket]
	if bucketUploads == nil {
		u.buckets[bucket] = newBucketUploads()
		bucketUploads = u.buckets[bucket]
	}

	bucketUploads.add(mpu)

	return mpu
}

func (u *uploader) ListParts(bucket, object string, uploadID UploadID, marker int, limit int64) (*ListMultipartUploadPartsResult, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	mpu, err := u.getUnlocked(bucket, object, uploadID)
	if err != nil {
		return nil, err
	}

	var result = ListMultipartUploadPartsResult{
		Bucket:           bucket,
		Key:              object,
		UploadID:         uploadID,
		MaxParts:         limit,
		PartNumberMarker: marker,
		StorageClass:     "STANDARD", // FIXME
	}

	var cnt int64
	for partNumber, part := range mpu.parts[marker:] {
		if part == nil {
			continue
		}

		if cnt >= limit {
			result.IsTruncated = true
			result.NextPartNumberMarker = partNumber
			break
		}

		result.Parts = append(result.Parts, ListMultipartUploadPartItem{
			ETag:         part.ETag,
			Size:         int64(len(part.Body)),
			PartNumber:   partNumber,
			LastModified: part.LastModified,
		})

		cnt++
	}

	return &result, nil
}

func (u *uploader) List(bucket string, marker *UploadListMarker, prefix Prefix, limit int64) (*ListMultipartUploadsResult, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	bucketUploads, ok := u.buckets[bucket]
	if !ok {
		return nil, ErrNoSuchUpload
	}

	var result = ListMultipartUploadsResult{
		Bucket:     bucket,
		Delimiter:  prefix.Delimiter,
		Prefix:     prefix.Prefix,
		MaxUploads: limit,
	}

	// we only need to use the uploadID to start the page if one was actually
	// supplied, otherwise assume we can start from the start of the iterator:
	var firstFound = true

	var iter = goskipiter.New(bucketUploads.objectIndex.Iterator())
	if marker != nil {
		iter.Seek(marker.Object)
		firstFound = marker.UploadID == ""
		result.UploadIDMarker = marker.UploadID
		result.KeyMarker = marker.Object
	}

	// Indicates whether the returned list of multipart uploads is truncated.
	// The list can be truncated if the number of multipart uploads exceeds
	// the limit allowed or specified by MaxUploads.
	//
	// In our case, this could be because there are still objects left in the
	// iterator, or because there are still uploadIDs left in the slice inside
	// the iteration.
	var truncated bool

	var cnt int64
	var seenPrefixes = map[string]bool{}
	var match PrefixMatch

	for iter.Next() {
		object := iter.Key().(string)
		uploads := iter.Value().([]*multipartUpload)

	retry:
		matched := prefix.Match(object, &match)
		if !matched {
			continue
		}

		if !firstFound {
			for idx, mpu := range uploads {
				if mpu.ID == marker.UploadID {
					firstFound = true
					uploads = uploads[idx:]
					goto retry
				}
			}

		} else {
			if match.CommonPrefix {
				if !seenPrefixes[match.MatchedPart] {
					result.CommonPrefixes = append(result.CommonPrefixes, match.AsCommonPrefix())
					seenPrefixes[match.MatchedPart] = true
				}

			} else {
				for idx, upload := range uploads {
					result.Uploads = append(result.Uploads, ListMultipartUploadItem{
						StorageClass: "STANDARD", // FIXME
						Key:          object,
						UploadID:     upload.ID,
						Initiated:    ContentTime{Time: upload.Initiated},
					})

					cnt++
					if cnt >= limit {
						if idx != len(uploads)-1 { // if this is not the last iteration, we have truncated
							truncated = true
							result.NextUploadIDMarker = uploads[idx+1].ID
							result.NextKeyMarker = object
						}
						goto done
					}
				}
			}
		}
	}

done:
	// If we did not truncate while in the middle of an object's upload ID list,
	// we need to see if there are more objects in the outer iteration:
	if !truncated {
		for iter.Next() {
			object := iter.Key().(string)
			if matched := prefix.Match(object, &match); matched && !match.CommonPrefix {
				truncated = true

				// This is not especially defensive; it assumes the rest of the code works
				// as it should. Could be something to clean up later:
				result.NextUploadIDMarker = iter.Value().([]*multipartUpload)[0].ID
				result.NextKeyMarker = object
				break
			}
		}
	}

	result.IsTruncated = truncated

	return &result, nil
}

func (u *uploader) Complete(bucket, object string, id UploadID) (*multipartUpload, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	up, err := u.getUnlocked(bucket, object, id)
	if err != nil {
		return nil, err
	}

	// if getUnlocked succeeded, so will this:
	u.buckets[bucket].remove(id)

	return up, nil
}

func (u *uploader) Get(bucket, object string, id UploadID) (mu *multipartUpload, err error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.getUnlocked(bucket, object, id)
}

func (u *uploader) getUnlocked(bucket, object string, id UploadID) (mu *multipartUpload, err error) {
	bucketUps, ok := u.buckets[bucket]
	if !ok {
		return nil, ErrNoSuchUpload
	}

	mu, ok = bucketUps.uploads[id]
	if !ok {
		return nil, ErrNoSuchUpload
	}

	if mu.Bucket != bucket || mu.Object != object {
		// FIXME: investigate what AWS does here; essentially if you initiate a
		// multipart upload at '/ObjectName1?uploads', then complete the upload
		// at '/ObjectName2?uploads', what happens?
		return nil, ErrNoSuchUpload
	}

	return mu, nil
}

// UploadListMarker is used to seek to the start of a page in a ListMultipartUploads operation.
type UploadListMarker struct {
	// Represents the key-marker query parameter. Together with 'uploadID',
	// this parameter specifies the multipart upload after which listing should
	// begin.
	//
	// If 'uploadID' is not specified, only the keys lexicographically greater
	// than the specified key-marker will be included in the list.
	//
	// If 'uploadID' is specified, any multipart uploads for a key equal to
	// 'object'  might also be included, provided those multipart uploads have
	// upload IDs lexicographically greater than the specified uploadID.
	Object string

	// Represents the upload-id-marker query parameter to the
	// ListMultipartUploads operation. Together with 'object', specifies the
	// multipart upload after which listing should begin. If 'object' is not
	// specified, the 'uploadID' parameter is ignored.
	UploadID UploadID
}

// uploadListMarkerFromQuery collects the upload-id-marker and key-marker query parameters
// to the ListMultipartUploads operation.
func uploadListMarkerFromQuery(q url.Values) *UploadListMarker {
	object := q.Get("key-marker")
	if object == "" {
		return nil
	}
	return &UploadListMarker{Object: object, UploadID: UploadID(q.Get("upload-id-marker"))}
}

type multipartUploadPart struct {
	PartNumber   int
	ETag         string
	Body         []byte
	LastModified ContentTime
}

type multipartUpload struct {
	ID        UploadID
	Bucket    string
	Object    string
	Meta      map[string]string
	Initiated time.Time

	// Part numbers are limited in S3 to 10,000, so we can be a little wasteful.
	// If a new part number is added, the slice is grown to that size. Depending
	// on how bad the input is, this could mean you have a 10,000 element slice
	// that is almost all nils. This shouldn't be a problem in practice.
	//
	// We need to use a slice here so we can get deterministic ordering in order
	// to support pagination when listing the upload parts.
	//
	// The minimum part ID is 1, which means the first item in this slice will
	// always be nil.
	//
	// Do not attempt to access parts without locking mu.
	parts []*multipartUploadPart

	mu sync.Mutex
}

func (mpu *multipartUpload) AddPart(partNumber int, at time.Time, body []byte) (etag string, err error) {
	if partNumber > MaxUploadPartNumber {
		return "", ErrInvalidPart
	}

	mpu.mu.Lock()
	defer mpu.mu.Unlock()

	// What the ETag actually is is not specified, so let's just invent any old thing
	// from guaranteed unique input:
	hash := md5.New()
	hash.Write([]byte(body))
	etag = fmt.Sprintf(`"%s"`, hex.EncodeToString(hash.Sum(nil)))

	part := multipartUploadPart{
		PartNumber:   partNumber,
		Body:         body,
		ETag:         etag,
		LastModified: NewContentTime(at),
	}
	if partNumber >= len(mpu.parts) {
		mpu.parts = append(mpu.parts, make([]*multipartUploadPart, partNumber-len(mpu.parts)+1)...)
	}
	mpu.parts[partNumber] = &part
	return etag, nil
}

func (mpu *multipartUpload) Reassemble(input *CompleteMultipartUploadRequest) (body []byte, etag string, err error) {
	mpu.mu.Lock()
	defer mpu.mu.Unlock()

	mpuPartsLen := len(mpu.parts)

	// FIXME: what does AWS do when mpu.Parts > input.Parts? Presumably you may
	// end up uploading more parts than you need to assemble, so it should
	// probably just ignore that?
	if len(input.Parts) > mpuPartsLen {
		return nil, "", ErrInvalidPart
	}

	if !input.partsAreSorted() {
		return nil, "", ErrInvalidPartOrder
	}

	var size int64

	for _, inPart := range input.Parts {
		if inPart.PartNumber >= mpuPartsLen || mpu.parts[inPart.PartNumber] == nil {
			return nil, "", ErrorMessagef(ErrInvalidPart, "unexpected part number %d in complete request", inPart.PartNumber)
		}

		upPart := mpu.parts[inPart.PartNumber]
		if strings.Trim(inPart.ETag, "\"") != strings.Trim(upPart.ETag, "\"") {
			return nil, "", ErrorMessagef(ErrInvalidPart, "unexpected part etag for number %d in complete request", inPart.PartNumber)
		}

		size += int64(len(upPart.Body))
	}

	body = make([]byte, 0, size)
	for _, part := range input.Parts {
		body = append(body, mpu.parts[part.PartNumber].Body...)
	}

	hash := fmt.Sprintf("%x", md5.Sum(body))

	return body, hash, nil
}
