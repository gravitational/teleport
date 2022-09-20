//go:build go1.16
// +build go1.16

// Copyright 2017 Microsoft Corporation. All rights reserved.
// Use of this source code is governed by an MIT
// license that can be found in the LICENSE file.

/*
Package azcore implements an HTTP request/response middleware pipeline.

The middleware consists of three components.

   - One or more Policy instances.
   - A Transport instance.
   - A Pipeline instance that combines the Policy and Transport instances.

Implementing the Policy Interface

A Policy can be implemented in two ways; as a first-class function for a stateless Policy, or as
a method on a type for a stateful Policy.  Note that HTTP requests made via the same pipeline share
the same Policy instances, so if a Policy mutates its state it MUST be properly synchronized to
avoid race conditions.

A Policy's Do method is called when an HTTP request wants to be sent over the network. The Do method can
perform any operation(s) it desires. For example, it can log the outgoing request, mutate the URL, headers,
and/or query parameters, inject a failure, etc.  Once the Policy has successfully completed its request
work, it must call the Next() method on the *azcore.Request instance in order to pass the request to the
next Policy in the chain.

When an HTTP response comes back, the Policy then gets a chance to process the response/error.  The Policy instance
can log the response, retry the operation if it failed due to a transient error or timeout, unmarshal the response
body, etc.  Once the Policy has successfully completed its response work, it must return the *azcore.Response
and error instances to its caller.

Template for implementing a stateless Policy:

   func NewMyStatelessPolicy() Policy {
      return azcore.PolicyFunc(func(req *azcore.Request) (*azcore.Response, error) {
         // TODO: mutate/process Request here

         // forward Request to next Policy & get Response/error
         resp, err := req.Next()

         // TODO: mutate/process Response/error here

         // return Response/error to previous Policy
         return resp, err
      })
   }

Template for implementing a stateful Policy:

   type MyStatefulPolicy struct {
      // TODO: add configuration/setting fields here
   }

   // TODO: add initialization args to NewMyStatefulPolicy()
   func NewMyStatefulPolicy() Policy {
      return &MyStatefulPolicy{
         // TODO: initialize configuration/setting fields here
      }
   }

   func (p *MyStatefulPolicy) Do(req *azcore.Request) (resp *azcore.Response, err error) {
         // TODO: mutate/process Request here

         // forward Request to next Policy & get Response/error
         resp, err := req.Next()

         // TODO: mutate/process Response/error here

         // return Response/error to previous Policy
         return resp, err
   }

Implementing the Transport Interface

The Transport interface is responsible for sending the HTTP request and returning the corresponding
HTTP response or error.  The Transport is invoked by the last Policy in the chain.  The default Transport
implementation uses a shared http.Client from the standard library.

The same stateful/stateless rules for Policy implementations apply to Transport implementations.

Using Policy and Transport Instances Via a Pipeline

To use the Policy and Transport instances, an application passes them to the NewPipeline function.

   func NewPipeline(transport Transport, policies ...Policy) Pipeline

The specified Policy instances form a chain and are invoked in the order provided to NewPipeline
followed by the Transport.

Once the Pipeline has been created, create a Request instance and pass it to Pipeline's Do method.

   func NewRequest(ctx context.Context, httpMethod string, endpoint string) (*Request, error)

   func (p Pipeline) Do(req *Request) (*Response, error)

The Pipeline.Do method sends the specified Request through the chain of Policy and Transport
instances.  The response/error is then sent through the same chain of Policy instances in reverse
order.  For example, assuming there are Policy types PolicyA, PolicyB, and PolicyC along with
TransportA.

   pipeline := NewPipeline(TransportA, PolicyA, PolicyB, PolicyC)

The flow of Request and Response looks like the following:

   azcore.Request -> PolicyA -> PolicyB -> PolicyC -> TransportA -----+
                                                                      |
                                                               HTTP(s) endpoint
                                                                      |
   caller <--------- PolicyA <- PolicyB <- PolicyC <- azcore.Response-+

Creating a Request Instance

The Request instance passed to Pipeline's Do method is a wrapper around an *http.Request.  It also
contains some internal state and provides various convenience methods.  You create a Request instance
by calling the NewRequest function:

   func NewRequest(ctx context.Context, httpMethod string, endpoint string) (*Request, error)

If the Request should contain a body, call the SetBody method.

   func (req *Request) SetBody(body ReadSeekCloser, contentType string) error

A seekable stream is required so that upon retry, the retry Policy instance can seek the stream
back to the beginning before retrying the network request and re-uploading the body.

Sending an Explicit Null

Operations like JSON-MERGE-PATCH send a JSON null to indicate a value should be deleted.

   {
      "delete-me": null
   }

This requirement conflicts with the SDK's default marshalling that specifies "omitempty" as
a means to resolve the ambiguity between a field to be excluded and its zero-value.

   type Widget struct {
      Name  *string `json:",omitempty"`
      Count *int    `json:",omitempty"`
   }

In the above example, Name and Count are defined as pointer-to-type to disambiguate between
a missing value (nil) and a zero-value (0) which might have semantic differences.

In a PATCH operation, any fields left as `nil` are to have their values preserved.  When updating
a Widget's count, one simply specifies the new value for Count, leaving Name nil.

To fulfill the requirement for sending a JSON null, the NullValue() function can be used.

   w := Widget{
      Count: azcore.NullValue(0).(*int),
   }

This sends an explict "null" for Count, indicating that any current value for Count should be deleted.

Processing the Response

When the HTTP response is received, the underlying *http.Response is wrapped in a Response type.
The Response type contains various convenience methods, like testing the HTTP response code and
unmarshalling the response body in a particular format.

The Response is returned through all the Policy instances. Each Policy instance can inspect/mutate the
embedded *http.Response.
*/
package azcore
