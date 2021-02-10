package main

import (
	"context"
	"log"
	"time"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/client"
	"github.com/gravitational/teleport/lib/services"
)

// accessWorkflow performs the necessary access management functions as an example
func accessWorkflow(ctx context.Context, client *client.Client) {
	// create access request for api-admin to temporarily use the admin role in the cluster
	accessReq, err := auth.NewAccessRequest("api-admin", "admin")
	if err != nil {
		log.Printf("Failed to make new access request: %v", err)
		return
	}

	// use AccessRequest setters to set optional fields
	accessReq.SetAccessExpiry(time.Now().Add(time.Hour))
	accessReq.SetRequestReason("I need more power.")
	accessReq.SetSystemAnnotations(map[string][]string{
		"ticket": []string{"137"},
	})

	if err = client.CreateAccessRequest(ctx, accessReq); err != nil {
		log.Printf("Failed to create access request: %v", err)
		return
	}

	log.Printf("Created Access Request: %v", accessReq)

	// defer deletion in case of an error below
	defer func() {
		// delete access request
		if err = client.DeleteAccessRequest(ctx, accessReq.GetName()); err != nil {
			log.Printf("Failed to delete access request: %v", err)
			return
		}

		log.Println("Deleted Access Request")
	}()

	// retrieve all pending access requests
	filter := services.AccessRequestFilter{State: services.RequestState_PENDING}
	accessReqs, err := client.GetAccessRequests(ctx, filter)
	if err != nil {
		log.Printf("Failed to retrieve access requests: %v", accessReqs)
		return
	}

	log.Println("Retrieved access requests:")
	for _, a := range accessReqs {
		log.Printf("  %v", a)
	}

	// approve access request
	if err = client.SetAccessRequestState(ctx, services.AccessRequestUpdate{
		RequestID: accessReq.GetName(),
		State:     services.RequestState_APPROVED,
		Reason:    "seems legit",
		// Roles: If you don't want to grant all the roles requested,
		// you can provide a subset of role with the Roles field.
	}); err != nil {
		log.Printf("Failed to accept request: %v", err)
		return
	}

	log.Println("Approved Access Request")

	// deny access request
	if err = client.SetAccessRequestState(ctx, services.AccessRequestUpdate{
		RequestID: accessReq.GetName(),
		State:     services.RequestState_DENIED,
		Reason:    "not today",
	}); err != nil {
		log.Printf("Failed to deny request: %v", err)
		return
	}

	log.Println("Denied Access Request")
}
