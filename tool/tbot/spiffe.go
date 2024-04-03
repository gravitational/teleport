/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

func onSPIFFEInspect(path string) error {
	ctx := context.Background()
	log.WithField("path", path).Info("Inspecting SPIFFE Workload API Endpoint")

	source, err := workloadapi.New(ctx, workloadapi.WithLogger(log), workloadapi.WithAddr(path))
	if err != nil {
		return trace.Wrap(err, "creating x509 source")
	}
	defer source.Close()

	res, err := source.FetchX509Context(ctx)
	if err != nil {
		return trace.Wrap(err, "getting x509 context")
	}

	log.
		WithField("svids_count", len(res.SVIDs)).
		WithField("bundles_count", res.Bundles.Len()).
		Info("Received X.509 SVID context from Workload API")

	if len(res.SVIDs) == 0 {
		log.Error("No SVIDs received, check your configuration.")
	} else {
		fmt.Println("SVIDS")
		for _, svid := range res.SVIDs {
			fmt.Printf("- %s\n", svid.ID.String())
			fmt.Printf("  - Hint: %s\n", svid.Hint)
			fmt.Printf("  - Expiry: %s\n", svid.Certificates[0].NotAfter)
			for _, san := range svid.Certificates[0].DNSNames {
				fmt.Printf("  - DNS SAN: %s\n", san)
			}
			for _, san := range svid.Certificates[0].IPAddresses {
				fmt.Printf("  - IP SAN: %s\n", san)
			}
		}
	}

	if res.Bundles.Len() == 0 {
		log.Error("No trust bundles received, check your configuration.")
	} else {
		fmt.Println("Trust Bundles")
		for _, bundle := range res.Bundles.Bundles() {
			fmt.Printf("- %s\n", bundle.TrustDomain())
		}
	}

	return nil
}
