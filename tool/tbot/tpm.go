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
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/tpm"
)

func onTPMIdentify() error {
	ctx := context.Background()
	data, err := tpm.Query(ctx, slog.Default())
	if err != nil {
		return trace.Wrap(err, "querying TPM")
	}
	fmt.Printf("TPM Information:\n")
	fmt.Printf("EKPub Hash: %s\n", data.EKPubHash)
	fmt.Printf("EKCert Detected: %t\n", data.EKCertPresent)
	if data.EKCertPresent {
		fmt.Printf("EKCert Serial: %s\n", data.EKCertSerial)
	}
	return nil
}
