package main

import (
	"context"
	"log"
	"time"

	"github.com/gravitational/teleport"
	auth "github.com/gravitational/teleport/lib/auth/client"
	"github.com/gravitational/teleport/lib/auth/server"
)

// tokenCRUD performs each token crud function as an example.
func tokenCRUD(ctx context.Context, client *auth.Client) {
	// create a randomly generated token for proxy servers to join the cluster with
	tokenString, err := client.GenerateToken(ctx, server.GenerateTokenRequest{
		Roles: teleport.Roles{teleport.RoleProxy},
		TTL:   time.Hour,
	})
	if err != nil {
		log.Printf("Failed to generate token: %v", err)
		return
	}

	log.Printf("Generated token: %v", tokenString)

	// defer deletion in case of an error below
	defer func() {
		// delete token
		if err = client.DeleteToken(ctx, tokenString); err != nil {
			log.Printf("Failed to delete token: %v", err)
		}

		log.Println("Deleted token")
	}()

	// retrieve token
	token, err := client.GetToken(ctx, tokenString)
	if err != nil {
		log.Printf("Failed to retrieve token for update: %v", err)
		return
	}

	log.Printf("Retrieved token: %v", token.GetName())

	// update the token to be expired
	token.SetExpiry(time.Now())
	if err = client.UpsertToken(ctx, token); err != nil {
		log.Printf("Failed to update token: %v", err)
		return
	}

	log.Println("Updated token")
}
