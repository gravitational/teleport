package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
)

// rolesCRUD performs each roles crud function as an example
func roleCRUD(ctx context.Context, client *auth.Client) error {
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
		return fmt.Errorf("Failed to make new role %v", err)
	}

	if err = client.UpsertRole(ctx, role); err != nil {
		return fmt.Errorf("Failed to create role: %v", err)
	}

	log.Printf("Created Role: %v", role.GetName())

	// retrieve auditor role
	role, err = client.GetRole("auditor")
	if err != nil {
		return fmt.Errorf("Failed to retrieve role for updating: %v", err)
	}

	log.Printf("Retrieved Role: %v", role.GetName())

	// update the auditor role's read rule to not provide access to secrets
	role.SetRules(services.Allow, []services.Rule{
		services.NewRule(services.KindSession, services.ReadNoSecrets()),
	})
	if err = client.UpsertRole(ctx, role); err != nil {
		return fmt.Errorf("Failed to update role: %v", err)
	}

	log.Printf("Updated role")

	// delete the auditor role we just created
	if err = client.DeleteRole(ctx, "auditor"); err != nil {
		return fmt.Errorf("Failed to delete role: %v", err)
	}

	log.Printf("Deleted role")

	return nil
}
