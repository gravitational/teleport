/*
Copyright 2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"fmt"
	"log"

	"github.com/gravitational/teleport/lib/services"
)

func main() {
	log.Printf("Starting teleport client...")
	client, err := connectClient()
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()

	ca, err := client.GetCertAuthority(services.CertAuthID{
		DomainName: "example.com",
		Type:       services.HostCA,
	}, false)

	fmt.Println(ca.GetRoles())

	fmt.Println("")
	err = roleCRUD(ctx, client)
	if err != nil {
		log.Print(err)
	}

	fmt.Println("")
	err = tokenCRUD(ctx, client)
	if err != nil {
		log.Print(err)
	}

	// fmt.Println("")
	// err = managingAccessRequests(ctx, client)
	// if err != nil {
	// 	log.Print(err)
	// }
}
