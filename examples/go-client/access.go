package main

import (
	"context"
	"fmt"
	"log"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
)

// managingAccessRequests performs the necessary access management functions as an example
func managingAccessRequests(ctx context.Context, client *auth.Client) error {
	// create a new access request for api-admin2 to use the api-admin role in the cluster
	accessReq, err := services.NewAccessRequest("api-admin", "api-admin")
	if err != nil {
		return fmt.Errorf("Failed to make new access request: %v", err)
	}

	err = client.CreateAccessRequest(ctx, accessReq)
	if err != nil {
		return fmt.Errorf("Failed to create access request: %v", err)
	}
	log.Printf("Created Access Request: %v", accessReq)

	// accept an access request
	err = client.SetAccessRequestState(ctx, accessReq.GetName(), services.RequestState_APPROVED)
	if err != nil {
		return fmt.Errorf("Failed to accept request: %v", err)
	}
	log.Printf("Approved Access Request: %v", accessReq)

	// deny an access request
	err = client.SetAccessRequestState(ctx, accessReq.GetName(), services.RequestState_DENIED)
	if err != nil {
		return fmt.Errorf("Failed to accept request: %v", err)
	}
	log.Printf("Denied Access Request: %v", accessReq)

	// retrieve all pending access requests
	accessReqs, err := client.GetAccessRequests(ctx, services.AccessRequestFilter{State: services.RequestState_PENDING})
	if err != nil {
		return fmt.Errorf("Failed to retrieve access requests: %v", accessReqs)
	}
	log.Println("Retrieved access requests:")
	for _, a := range accessReqs {
		log.Printf("  %v", a)
	}

	return nil
}
