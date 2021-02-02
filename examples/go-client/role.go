package main

import (
	"context"
	"log"
	"time"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
)

// rolesCRUD performs each roles crud function as an example
func roleCRUD(ctx context.Context, client *auth.Client) {
	// create a new auditor role which has very limited permissions
	role, err := services.NewRole("auditor", services.RoleSpecV3{
		Options: services.RoleOptions{
			MaxSessionTTL: services.Duration(time.Hour),
		},
		Allow: services.RoleConditions{
			Logins: []string{"auditor"},
			Rules: []services.Rule{
				services.NewRule(services.KindSession, services.RO()),
			},
		},
		Deny: services.RoleConditions{
			NodeLabels: services.Labels{"*": []string{"*"}},
		},
	})
	if err != nil {
		log.Printf("Failed to make new role %v", err)
		return
	}

	if err = client.UpsertRole(ctx, role); err != nil {
		log.Printf("Failed to create role: %v", err)
		return
	}

	log.Printf("Created Role: %v", role.GetName())

	// defer deletion in case of an error below
	defer func() {
		// delete the auditor role we just created
		if err = client.DeleteRole(ctx, "auditor"); err != nil {
			log.Printf("Failed to delete role: %v", err)
		}

		log.Printf("Deleted role")
	}()

	// retrieve auditor role
	role, err = client.GetRole("auditor")
	if err != nil {
		log.Printf("Failed to retrieve role for updating: %v", err)
		return
	}

	log.Printf("Retrieved Role: %v", role.GetName())

	// update the auditor role's ttl to one day
	role.SetOptions(services.RoleOptions{
		MaxSessionTTL: services.Duration(time.Hour * 24),
	})
	if err = client.UpsertRole(ctx, role); err != nil {
		log.Printf("Failed to update role: %v", err)
		return
	}

	log.Printf("Updated role")
}
