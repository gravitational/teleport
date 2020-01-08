package gofakes3

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"time"
)

// Error codes are documented here:
// https://docs.aws.amazon.com/AmazonS3/latest/API/ErrorResponses.html
//
// If you add a code to this list, please also add it to ErrorCode.Status().
//
const (
	ErrNone ErrorCode = ""

	// The Content-MD5 you specified did not match what we received.
	ErrBadDigest ErrorCode = "BadDigest"

	ErrBucketAlreadyExists ErrorCode = "BucketAlreadyExists"

	// Raised when attempting to delete a bucket that still contains items.
	ErrBucketNotEmpty ErrorCode = "BucketNotEmpty"

	// "Indicates that the versioning configuration specified in the request is invalid"
	ErrIllegalVersioningConfiguration ErrorCode = "IllegalVersioningConfigurationException"

	// You did not provide the number of bytes specified by the Content-Length
	// HTTP header:
	ErrIncompleteBody ErrorCode = "IncompleteBody"

	// POST requires exactly one file upload per request.
	ErrIncorrectNumberOfFilesInPostRequest ErrorCode = "IncorrectNumberOfFilesInPostRequest"

	// InlineDataTooLarge occurs when using the PutObjectInline method of the
	// SOAP interface
	// (https://docs.aws.amazon.com/AmazonS3/latest/API/SOAPPutObjectInline.html).
	// This is not documented on the errors page; the error is included here
	// only for reference.
	ErrInlineDataTooLarge ErrorCode = "InlineDataTooLarge"

	ErrInvalidArgument ErrorCode = "InvalidArgument"

	// https://docs.aws.amazon.com/AmazonS3/latest/dev/BucketRestrictions.html#bucketnamingrules
	ErrInvalidBucketName ErrorCode = "InvalidBucketName"

	// The Content-MD5 you specified is not valid.
	ErrInvalidDigest ErrorCode = "InvalidDigest"

	ErrInvalidRange         ErrorCode = "InvalidRange"
	ErrInvalidToken         ErrorCode = "InvalidToken"
	ErrKeyTooLong           ErrorCode = "KeyTooLongError" // This is not a typo: Error is part of the string, but redundant in the constant name
	ErrMalformedPOSTRequest ErrorCode = "MalformedPOSTRequest"

	// One or more of the specified parts could not be found. The part might
	// not have been uploaded, or the specified entity tag might not have
	// matched the part's entity tag.
	ErrInvalidPart ErrorCode = "InvalidPart"

	// The list of parts was not in ascending order. Parts list must be
	// specified in order by part number.
	ErrInvalidPartOrder ErrorCode = "InvalidPartOrder"

	ErrInvalidURI ErrorCode = "InvalidURI"

	ErrMetadataTooLarge ErrorCode = "MetadataTooLarge"
	ErrMethodNotAllowed ErrorCode = "MethodNotAllowed"
	ErrMalformedXML     ErrorCode = "MalformedXML"

	// You must provide the Content-Length HTTP header.
	ErrMissingContentLength ErrorCode = "MissingContentLength"

	// See BucketNotFound() for a helper function for this error:
	ErrNoSuchBucket ErrorCode = "NoSuchBucket"

	// See KeyNotFound() for a helper function for this error:
	ErrNoSuchKey ErrorCode = "NoSuchKey"

	// The specified multipart upload does not exist. The upload ID might be
	// invalid, or the multipart upload might have been aborted or completed.
	ErrNoSuchUpload ErrorCode = "NoSuchUpload"

	ErrNoSuchVersion ErrorCode = "NoSuchVersion"

	ErrRequestTimeTooSkewed ErrorCode = "RequestTimeTooSkewed"
	ErrTooManyBuckets       ErrorCode = "TooManyBuckets"
	ErrNotImplemented       ErrorCode = "NotImplemented"

	ErrInternal ErrorCode = "InternalError"
)

// INTERNAL errors! These are not part of the S3 interface, they are codes
// we have declared ourselves. Should all map to a 500 status code:
const (
	ErrInternalPageNotImplemented InternalErrorCode = "PaginationNotImplemented"
)

// errorResponse should be implemented by any type that needs to be handled by
// ensureErrorResponse.
type errorResponse interface {
	Error
	enrich(requestID string)
}

func ensureErrorResponse(err error, requestID string) Error {
	switch err := err.(type) {
	case errorResponse:
		err.enrich(requestID)
		return err

	case ErrorCode:
		return &ErrorResponse{
			Code:      err,
			RequestID: requestID,
			Message:   string(err),
		}

	default:
		return &ErrorResponse{
			Code:      ErrInternal,
			Message:   "Internal Error",
			RequestID: requestID,
		}
	}
}

type Error interface {
	error
	ErrorCode() ErrorCode
}

// ErrorResponse is the base error type returned by S3 when any error occurs.
//
// Some errors contain their own additional fields in the response, for example
// ErrRequestTimeTooSkewed, which contains the server time and the skew limit.
// To create one of these responses, subclass it (but please don't export it):
//
//	type notQuiteRightResponse struct {
//		ErrorResponse
//		ExtraField int
//	}
//
// Next, create a constructor that populates the error. Interfaces won't work
// for this job as the error itself does double-duty as the XML response
// object. Fill the struct out however you please, but don't forget to assign
// Code and Message:
//
//	func NotQuiteRight(at time.Time, max time.Duration) error {
// 	    code := ErrNotQuiteRight
// 	    return &notQuiteRightResponse{
// 	        ErrorResponse{Code: code, Message: code.Message()},
// 	        123456789,
// 	    }
// 	}
//
type ErrorResponse struct {
	XMLName xml.Name `xml:"Error"`

	Code      ErrorCode
	Message   string `xml:",omitempty"`
	RequestID string `xml:"RequestId,omitempty"`
	HostID    string `xml:"HostId,omitempty"`
}

func (e *ErrorResponse) ErrorCode() ErrorCode { return e.Code }

func (e *ErrorResponse) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (r *ErrorResponse) enrich(requestID string) {
	r.RequestID = requestID
}

func ErrorMessage(code ErrorCode, message string) error {
	return &ErrorResponse{Code: code, Message: message}
}

func ErrorMessagef(code ErrorCode, message string, args ...interface{}) error {
	return &ErrorResponse{Code: code, Message: fmt.Sprintf(message, args...)}
}

type ErrorInvalidArgumentResponse struct {
	ErrorResponse

	ArgumentName  string `xml:"ArgumentName"`
	ArgumentValue string `xml:"ArgumentValue"`
}

func ErrorInvalidArgument(name, value, message string) error {
	return &ErrorInvalidArgumentResponse{
		ErrorResponse: ErrorResponse{Code: ErrInvalidArgument, Message: message},
		ArgumentName:  name, ArgumentValue: value}
}

// ErrorCode represents an S3 error code, documented here:
// https://docs.aws.amazon.com/AmazonS3/latest/API/ErrorResponses.html
type ErrorCode string

func (e ErrorCode) ErrorCode() ErrorCode { return e }
func (e ErrorCode) Error() string        { return string(e) }

// InternalErrorCode represents an GoFakeS3 error code. It maps to ErrInternal
// when constructing a response.
type InternalErrorCode string

func (e InternalErrorCode) ErrorCode() ErrorCode { return ErrInternal }
func (e InternalErrorCode) Error() string        { return string(ErrInternal) }

// Message tries to return the same string as S3 would return for the error
// response, when it is known, or nothing when it is not. If you see the status
// text for a code we don't have listed in here in the wild, please let us
// know!
func (e ErrorCode) Message() string {
	switch e {
	case ErrInvalidBucketName:
		return `Bucket name must match the regex "^[a-zA-Z0-9.\-_]{1,255}$"`
	case ErrNoSuchBucket:
		return "The specified bucket does not exist"
	case ErrRequestTimeTooSkewed:
		return "The difference between the request time and the current time is too large"
	case ErrMalformedXML:
		return "The XML you provided was not well-formed or did not validate against our published schema"
	default:
		return ""
	}
}

func (e ErrorCode) Status() int {
	switch e {
	case ErrBucketAlreadyExists,
		ErrBucketNotEmpty:
		return http.StatusConflict

	case ErrBadDigest,
		ErrIllegalVersioningConfiguration,
		ErrIncompleteBody,
		ErrIncorrectNumberOfFilesInPostRequest,
		ErrInlineDataTooLarge,
		ErrInvalidArgument,
		ErrInvalidBucketName,
		ErrInvalidDigest,
		ErrInvalidPart,
		ErrInvalidPartOrder,
		ErrInvalidToken,
		ErrInvalidURI,
		ErrKeyTooLong,
		ErrMetadataTooLarge,
		ErrMethodNotAllowed,
		ErrMalformedPOSTRequest,
		ErrMalformedXML,
		ErrTooManyBuckets:
		return http.StatusBadRequest

	case ErrRequestTimeTooSkewed:
		return http.StatusForbidden

	case ErrInvalidRange:
		return http.StatusRequestedRangeNotSatisfiable

	case ErrNoSuchBucket,
		ErrNoSuchKey,
		ErrNoSuchUpload,
		ErrNoSuchVersion:
		return http.StatusNotFound

	case ErrNotImplemented:
		return http.StatusNotImplemented

	case ErrMissingContentLength:
		return http.StatusLengthRequired

	case ErrInternal:
		return http.StatusInternalServerError
	}

	return http.StatusInternalServerError
}

// HasErrorCode asserts that the error has a specific error code:
//
//	if HasErrorCode(err, ErrNoSuchBucket) {
//		// handle condition
//	}
//
// If err is nil and code is ErrNone, HasErrorCode returns true.
//
func HasErrorCode(err error, code ErrorCode) bool {
	if err == nil && code == "" {
		return true
	}
	s3err, ok := err.(interface{ ErrorCode() ErrorCode })
	if !ok {
		return false
	}
	return s3err.ErrorCode() == code
}

// IsAlreadyExists asserts that the error is a kind that indicates the resource
// already exists, similar to os.IsExist.
func IsAlreadyExists(err error) bool {
	return HasErrorCode(err, ErrBucketAlreadyExists)
}

type resourceErrorResponse struct {
	ErrorResponse
	Resource string
}

var _ errorResponse = &resourceErrorResponse{}

func ResourceError(code ErrorCode, resource string) error {
	return &resourceErrorResponse{
		ErrorResponse{Code: code, Message: code.Message()},
		resource,
	}
}

func BucketNotFound(bucket string) error { return ResourceError(ErrNoSuchBucket, bucket) }
func KeyNotFound(key string) error       { return ResourceError(ErrNoSuchKey, key) }

type requestTimeTooSkewedResponse struct {
	ErrorResponse
	ServerTime                 time.Time
	MaxAllowedSkewMilliseconds durationAsMilliseconds
}

var _ errorResponse = &requestTimeTooSkewedResponse{}

func requestTimeTooSkewed(at time.Time, max time.Duration) error {
	code := ErrRequestTimeTooSkewed
	return &requestTimeTooSkewedResponse{
		ErrorResponse{Code: code, Message: code.Message()},
		at, durationAsMilliseconds(max),
	}
}

// durationAsMilliseconds tricks xml.Marsha into serialising a time.Duration as
// truncated milliseconds instead of nanoseconds.
type durationAsMilliseconds time.Duration

func (m durationAsMilliseconds) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	var s = fmt.Sprintf("%d", time.Duration(m)/time.Millisecond)
	return e.EncodeElement(s, start)
}
