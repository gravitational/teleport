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
	"time"

	"github.com/gravitational/trace"
	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"

	"github.com/gravitational/teleport/lib/utils"
)

func onSPIFFEInspect(ctx context.Context, path string) error {
	log.InfoContext(ctx, "Inspecting SPIFFE Workload API Endpoint", "path", path)

	source, err := workloadapi.New(
		ctx,
		// TODO(noah): Upstream PR to add slog<->workloadapi.Logger adapter.
		workloadapi.WithLogger(utils.NewLogger()),
		workloadapi.WithAddr(path),
	)
	if err != nil {
		return trace.Wrap(err, "creating x509 source")
	}
	defer source.Close()

	x509Ctx, err := source.FetchX509Context(ctx)
	if err != nil {
		return trace.Wrap(err, "getting x509 context")
	}

	audience := "example.com"
	jwtSVIDs, err := source.FetchJWTSVIDs(ctx, jwtsvid.Params{
		Audience: audience,
	})
	if err != nil {
		return trace.Wrap(err, "getting JWT SVIDs")
	}

	jwtBundles, err := source.FetchJWTBundles(ctx)
	if err != nil {
		return trace.Wrap(err, "getting JWT bundles")
	}

	fmt.Println("# X509 SVIDs")
	if len(x509Ctx.SVIDs) == 0 {
		fmt.Println("No X509 SVIDs received, check your configuration")
	}
	for _, svid := range x509Ctx.SVIDs {
		fmt.Printf("- %s\n", svid.ID.String())
		fmt.Printf("  - Hint: %s\n", svid.Hint)
		fmt.Printf(
			"  - Expiry: %s (%s)\n",
			svid.Certificates[0].NotAfter,
			time.Until(svid.Certificates[0].NotAfter),
		)
		for _, san := range svid.Certificates[0].DNSNames {
			fmt.Printf("  - DNS SAN: %s\n", san)
		}
		for _, san := range svid.Certificates[0].IPAddresses {
			fmt.Printf("  - IP SAN: %s\n", san)
		}
	}

	fmt.Println("# X509 Trust Bundles")
	if x509Ctx.Bundles.Len() == 0 {
		fmt.Println("No X509 trust bundles received, check your configuration")
	}
	for _, bundle := range x509Ctx.Bundles.Bundles() {
		fmt.Printf("- %s (#CAs: %d)\n", bundle.TrustDomain(), len(bundle.X509Authorities()))
	}

	fmt.Println("# JWT SVIDs")
	if jwtBundles.Len() == 0 {
		fmt.Println("No JWT SVIDs received, check your configuration")
	}
	for _, jwtSVID := range jwtSVIDs {
		fmt.Printf("- %s\n", jwtSVID.ID)
		fmt.Printf("  - Expiry: %s (%s)\n", jwtSVID.Expiry, time.Until(jwtSVID.Expiry))
		fmt.Printf("  - Audiences: %v\n", jwtSVID.Audience)
		fmt.Printf("  - Hint: %s\n", jwtSVID.Hint)
		fmt.Printf("  - Value: %s\n", jwtSVID.Marshal())

		// Validate the produced JWT against the Workload API to validate that
		// ValidateJWTSVID is working as expected, and that the produced JWT is
		// valid.
		_, err := source.ValidateJWTSVID(ctx, jwtSVID.Marshal(), audience)
		if err != nil {
			fmt.Printf("  - Validation: FAIL - %v\n", err)
		} else {
			fmt.Printf("  - Validation: PASS\n")
		}
	}

	fmt.Println("# JWT Bundles")
	if jwtBundles.Len() == 0 {
		fmt.Println("No JWT trust bundles received, check your configuration")
	}
	for _, jwtBundle := range jwtBundles.Bundles() {
		fmt.Printf("- %s (#Keys: %d)\n", jwtBundle.TrustDomain(), len(jwtBundle.JWTAuthorities()))
		for keyID := range jwtBundle.JWTAuthorities() {
			fmt.Printf("  - Key ID: %s\n", keyID)
		}
	}

	return nil
}
