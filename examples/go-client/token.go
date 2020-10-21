package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
)

// tokenCRUD performs each token crud function as an example.
func tokenCRUD(ctx context.Context, client *auth.Client) {
	tokenString, err := client.GenerateToken(ctx, auth.GenerateTokenRequest{
		// You can set token explicitly, otherwise it will be generated
		// Token: CryptoRandomHex()
		// https://sosedoff.com/2014/12/15/generate-random-hex-string-in-go.html
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
		if err = client.DeleteToken(tokenString); err != nil {
			log.Printf("Failed to delete token: %v", err)
		}

		log.Println("Deleted token")
	}()

	// retrieve token
	token, err := client.GetToken(tokenString)
	if err != nil {
		log.Printf("Failed to retrieve token for update: %v", err)
		return
	}

	log.Printf("Retrieved token: %v", token.GetName())

	// update the token to be a trusted cluster join token
	token.SetRoles(teleport.Roles{teleport.RoleTrustedCluster})
	if err = client.UpsertToken(token); err != nil {
		log.Printf("Failed to update token: %v", err)
		return
	}

	log.Println("Updated token")
}

// Helper function to generate random tokens
func randomHex(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
