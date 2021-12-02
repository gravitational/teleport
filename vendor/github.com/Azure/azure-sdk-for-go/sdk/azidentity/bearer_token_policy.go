// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package azidentity

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
)

const (
	bearerTokenPrefix = "Bearer "
)

type bearerTokenPolicy struct {
	// mainResource is the resource to be retreived using the tenant specified in the credential
	mainResource *expiringResource
	// auxResources are additional resources that are required for cross-tenant applications
	auxResources map[string]*expiringResource
	// the following fields are read-only
	creds   azcore.TokenCredential
	options policy.TokenRequestOptions
}

type expiringResource struct {
	// cond is used to synchronize access to the shared resource embodied by the remaining fields
	cond *sync.Cond

	// acquiring indicates that some thread/goroutine is in the process of acquiring/updating the resource
	acquiring bool

	// resource contains the value of the shared resource
	resource interface{}

	// expiration indicates when the shared resource expires; it is 0 if the resource was never acquired
	expiration time.Time

	// acquireResource is the callback function that actually acquires the resource
	acquireResource acquireResource
}

type acquireResource func(state interface{}) (newResource interface{}, newExpiration time.Time, err error)

type acquiringResourceState struct {
	req *policy.Request
	p   bearerTokenPolicy
}

// acquire acquires or updates the resource; only one
// thread/goroutine at a time ever calls this function
func acquire(state interface{}) (newResource interface{}, newExpiration time.Time, err error) {
	s := state.(acquiringResourceState)
	tk, err := s.p.creds.GetToken(s.req.Raw().Context(), s.p.options)
	if err != nil {
		return nil, time.Time{}, err
	}
	return tk, tk.ExpiresOn, nil
}

func newExpiringResource(ar acquireResource) *expiringResource {
	return &expiringResource{cond: sync.NewCond(&sync.Mutex{}), acquireResource: ar}
}

func (er *expiringResource) GetResource(state interface{}) (interface{}, error) {
	// If the resource is expiring within this time window, update it eagerly.
	// This allows other threads/goroutines to keep running by using the not-yet-expired
	// resource value while one thread/goroutine updates the resource.
	const window = 2 * time.Minute // This example updates the resource 2 minutes prior to expiration

	now, acquire, resource := time.Now(), false, er.resource
	// acquire exclusive lock
	er.cond.L.Lock()
	for {
		if er.expiration.IsZero() || er.expiration.Before(now) {
			// The resource was never acquired or has expired
			if !er.acquiring {
				// If another thread/goroutine is not acquiring/updating the resource, this thread/goroutine will do it
				er.acquiring, acquire = true, true
				break
			}
			// Getting here means that this thread/goroutine will wait for the updated resource
		} else if er.expiration.Add(-window).Before(now) {
			// The resource is valid but is expiring within the time window
			if !er.acquiring {
				// If another thread/goroutine is not acquiring/renewing the resource, this thread/goroutine will do it
				er.acquiring, acquire = true, true
				break
			}
			// This thread/goroutine will use the existing resource value while another updates it
			resource = er.resource
			break
		} else {
			// The resource is not close to expiring, this thread/goroutine should use its current value
			resource = er.resource
			break
		}
		// If we get here, wait for the new resource value to be acquired/updated
		er.cond.Wait()
	}
	er.cond.L.Unlock() // Release the lock so no threads/goroutines are blocked

	var err error
	if acquire {
		// This thread/goroutine has been selected to acquire/update the resource
		var expiration time.Time
		resource, expiration, err = er.acquireResource(state)

		// Atomically, update the shared resource's new value & expiration.
		er.cond.L.Lock()
		if err == nil {
			// No error, update resource & expiration
			er.resource, er.expiration = resource, expiration
		}
		er.acquiring = false // Indicate that no thread/goroutine is currently acquiring the resrouce

		// Wake up any waiting threads/goroutines since there is a resource they can ALL use
		er.cond.L.Unlock()
		er.cond.Broadcast()
	}
	return resource, err // Return the resource this thread/goroutine can use
}

func newBearerTokenPolicy(creds azcore.TokenCredential, opts runtime.AuthenticationOptions) *bearerTokenPolicy {
	p := &bearerTokenPolicy{
		creds:        creds,
		options:      opts.TokenRequest,
		mainResource: newExpiringResource(acquire),
	}
	if len(opts.AuxiliaryTenants) > 0 {
		p.auxResources = map[string]*expiringResource{}
	}
	for _, t := range opts.AuxiliaryTenants {
		p.auxResources[t] = newExpiringResource(acquire)

	}
	return p
}

func (b *bearerTokenPolicy) Do(req *policy.Request) (*http.Response, error) {
	as := acquiringResourceState{
		p:   *b,
		req: req,
	}
	tk, err := b.mainResource.GetResource(as)
	if err != nil {
		return nil, err
	}
	if token, ok := tk.(*azcore.AccessToken); ok {
		req.Raw().Header.Set(headerXmsDate, time.Now().UTC().Format(http.TimeFormat))
		req.Raw().Header.Set(headerAuthorization, fmt.Sprintf("Bearer %s", token.Token))
	}
	auxTokens := []string{}
	for tenant, er := range b.auxResources {
		bCopy := *b
		bCopy.options.TenantID = tenant
		auxAS := acquiringResourceState{
			p:   bCopy,
			req: req,
		}
		auxTk, err := er.GetResource(auxAS)
		if err != nil {
			return nil, err
		}
		auxTokens = append(auxTokens, fmt.Sprintf("%s%s", bearerTokenPrefix, auxTk.(*azcore.AccessToken).Token))
	}
	if len(auxTokens) > 0 {
		req.Raw().Header.Set(headerAuxiliaryAuthorization, strings.Join(auxTokens, ", "))
	}
	return req.Next()
}
