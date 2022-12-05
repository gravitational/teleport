/*
Copyright 2022 Gravitational, Inc.

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
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/gravitational/teleport/api/client"
	apidefaults "github.com/gravitational/teleport/api/defaults"
)

// A new identity file can be generated with tsh or tctl.
//
//	$ tsh login --user=api-user --out=identity-file-path
//	$ tctl auth sign --user=api-user --out=identity-file-path
//
// The identity file's time to live can be specified with --ttl.

// Build the app with:
//
// go build .
//
// Run:
//
// ./print-nodes -identityFile=identity.out root.example.com:3080 leaf.example.com:3080

func main() {
	identityFile := flag.String("identityFile", "", "Path to the identity file")
	flag.Parse()

	if *identityFile == "" {
		log.Fatal("Missing identity file.")
	}

	proxyAddrs := flag.Args()
	if len(proxyAddrs) == 0 {
		log.Fatal("Missing proxy address. At least one is required.")
	}

	if err := listNodes(*identityFile, proxyAddrs); err != nil {
		log.Fatal(err)
	}
}

// listNode prints all nodes connected to proxies provided as proxyAddresses.
func listNodes(identityFile string, proxyAddresses []string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	for _, addr := range proxyAddresses {
		if err := listNode(ctx, identityFile, addr); err != nil {
			return err
		}
	}

	return nil
}

// listNode prints all nodes connected to Teleport Proxy provided as proxyAddress.
func listNode(ctx context.Context, identityFile, proxyAddress string) error {
	clt, err := client.New(ctx, client.Config{
		DialTimeout: 20 * time.Second,
		Context:     ctx,
		Addrs:       []string{proxyAddress},
		Credentials: []client.Credentials{
			client.LoadIdentityFile(identityFile),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer clt.Close()

	nodes, err := clt.GetNodes(ctx, apidefaults.Namespace)
	if err != nil {
		return fmt.Errorf("failed to fetch nodes from proxy %s, error: %w", proxyAddress, err)
	}

	log.Printf("nodes: %+v", nodes)

	return nil
}
