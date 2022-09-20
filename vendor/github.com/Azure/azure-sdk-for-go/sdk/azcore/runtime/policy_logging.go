//go:build go1.16
// +build go1.16

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package runtime

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/internal/shared"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/internal/diag"
	"github.com/Azure/azure-sdk-for-go/sdk/internal/log"
)

type logPolicy struct {
	options policy.LogOptions
}

// NewLogPolicy creates a RequestLogPolicy object configured using the specified options.
// Pass nil to accept the default values; this is the same as passing a zero-value options.
func NewLogPolicy(o *policy.LogOptions) policy.Policy {
	if o == nil {
		o = &policy.LogOptions{}
	}
	return &logPolicy{options: *o}
}

// logPolicyOpValues is the struct containing the per-operation values
type logPolicyOpValues struct {
	try   int32
	start time.Time
}

func (p *logPolicy) Do(req *policy.Request) (*http.Response, error) {
	// Get the per-operation values. These are saved in the Message's map so that they persist across each retry calling into this policy object.
	var opValues logPolicyOpValues
	if req.OperationValue(&opValues); opValues.start.IsZero() {
		opValues.start = time.Now() // If this is the 1st try, record this operation's start time
	}
	opValues.try++ // The first try is #1 (not #0)
	req.SetOperationValue(opValues)

	// Log the outgoing request as informational
	if log.Should(log.Request) {
		b := &bytes.Buffer{}
		fmt.Fprintf(b, "==> OUTGOING REQUEST (Try=%d)\n", opValues.try)
		writeRequestWithResponse(b, req, nil, nil)
		var err error
		if p.options.IncludeBody {
			err = writeReqBody(req, b)
		}
		log.Write(log.Request, b.String())
		if err != nil {
			return nil, err
		}
	}

	// Set the time for this particular retry operation and then Do the operation.
	tryStart := time.Now()
	response, err := req.Next() // Make the request
	tryEnd := time.Now()
	tryDuration := tryEnd.Sub(tryStart)
	opDuration := tryEnd.Sub(opValues.start)

	if log.Should(log.Response) {
		// We're going to log this; build the string to log
		b := &bytes.Buffer{}
		fmt.Fprintf(b, "==> REQUEST/RESPONSE (Try=%d/%v, OpTime=%v) -- ", opValues.try, tryDuration, opDuration)
		if err != nil { // This HTTP request did not get a response from the service
			fmt.Fprint(b, "REQUEST ERROR\n")
		} else {
			fmt.Fprint(b, "RESPONSE RECEIVED\n")
		}

		writeRequestWithResponse(b, req, response, err)
		if err != nil {
			// skip frames runtime.Callers() and runtime.StackTrace()
			b.WriteString(diag.StackTrace(2, 32))
		} else if p.options.IncludeBody {
			err = writeRespBody(response, b)
		}
		log.Write(log.Response, b.String())
	}
	return response, err
}

// returns true if the request/response body should be logged.
// this is determined by looking at the content-type header value.
func shouldLogBody(b *bytes.Buffer, contentType string) bool {
	contentType = strings.ToLower(contentType)
	if strings.HasPrefix(contentType, "text") ||
		strings.Contains(contentType, "json") ||
		strings.Contains(contentType, "xml") {
		return true
	}
	fmt.Fprintf(b, "   Skip logging body for %s\n", contentType)
	return false
}

// writes to a buffer, used for logging purposes
func writeReqBody(req *policy.Request, b *bytes.Buffer) error {
	if req.Raw().Body == nil {
		fmt.Fprint(b, "   Request contained no body\n")
		return nil
	}
	if ct := req.Raw().Header.Get(shared.HeaderContentType); !shouldLogBody(b, ct) {
		return nil
	}
	body, err := ioutil.ReadAll(req.Raw().Body)
	if err != nil {
		fmt.Fprintf(b, "   Failed to read request body: %s\n", err.Error())
		return err
	}
	if err := req.RewindBody(); err != nil {
		return err
	}
	logBody(b, body)
	return nil
}

// writes to a buffer, used for logging purposes
func writeRespBody(resp *http.Response, b *bytes.Buffer) error {
	ct := resp.Header.Get(shared.HeaderContentType)
	if ct == "" {
		fmt.Fprint(b, "   Response contained no body\n")
		return nil
	} else if !shouldLogBody(b, ct) {
		return nil
	}
	body, err := Payload(resp)
	if err != nil {
		fmt.Fprintf(b, "   Failed to read response body: %s\n", err.Error())
		return err
	}
	if len(body) > 0 {
		logBody(b, body)
	} else {
		fmt.Fprint(b, "   Response contained no body\n")
	}
	return nil
}

func logBody(b *bytes.Buffer, body []byte) {
	fmt.Fprintln(b, "   --------------------------------------------------------------------------------")
	fmt.Fprintln(b, string(body))
	fmt.Fprintln(b, "   --------------------------------------------------------------------------------")
}
