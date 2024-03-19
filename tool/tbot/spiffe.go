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
		log.Info("SVIDS")
	}
	for _, svid := range res.SVIDs {
		log.Infof("- %s", svid.ID.String())
		log.Infof("  - Hint: %s", svid.Hint)
		log.Infof("  - Expiry: %s", svid.Certificates[0].NotAfter)
		for _, san := range svid.Certificates[0].DNSNames {
			log.Infof("  - DNS SAN: %s", san)
		}
		for _, san := range svid.Certificates[0].IPAddresses {
			log.Infof("  - IP SAN: %s", san)
		}
	}

	if res.Bundles.Len() == 0 {
		log.Error("No trust bundles received, check your configuration.")
	} else {
		log.Info("Trust Bundles")
	}
	for _, bundle := range res.Bundles.Bundles() {
		log.Infof("- %s", bundle.TrustDomain())
	}

	return nil
}
