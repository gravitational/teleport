/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package conntest

import (
	"context"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
)

// ExternalAuditStorageConnectionTesterConfig defines the config fields for ExternalAuditStorageConnectionTester.
type ExternalAuditStorageConnectionTesterConfig struct {
	// UserClient is an auth client that has a User's identity.
	UserClient authclient.ClientI
}

// ExternalAuditStorageConnectionTester implements the ConnectionTester interface for testing External Audit Storage access.
type ExternalAuditStorageConnectionTester struct {
	cfg ExternalAuditStorageConnectionTesterConfig
}

// NewDatabaseConnectionTester returns a new DatabaseConnectionTester.
func NewExternalAuditStorageConnectionTester(cfg ExternalAuditStorageConnectionTesterConfig) (*ExternalAuditStorageConnectionTester, error) {
	return &ExternalAuditStorageConnectionTester{
		cfg,
	}, nil
}

// TestConnection tests the current configured ExternalAuditStorage draft by:
// * Uploading a dummy file to both the audit events and session recordings S3 Buckets.
// * Tests get object on the session recordings bucket.
// * Tests the retrieval of the Glue table.
// * Runs a test query against the audit events bucket through Athena.
func (s *ExternalAuditStorageConnectionTester) TestConnection(ctx context.Context, req TestConnectionRequest) (types.ConnectionDiagnostic, error) {
	if req.ResourceKind != types.KindExternalAuditStorage {
		return nil, trace.BadParameter("invalid value for ResourceKind, expected %q got %q", types.KindExternalAuditStorage, req.ResourceKind)
	}

	connectionDiagnosticID := uuid.NewString()
	connectionDiagnostic, err := types.NewConnectionDiagnosticV1(
		connectionDiagnosticID,
		map[string]string{},
		types.ConnectionDiagnosticSpecV1{
			// We start with a failed state so that we don't need to set it to each return statement once an error is returned.
			// if the test reaches the end, we force the test to be a success by calling
			// 	connectionDiagnostic.SetMessage(types.DiagnosticMessageSuccess)
			//	connectionDiagnostic.SetSuccess(true)
			Message: types.DiagnosticMessageFailed,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.cfg.UserClient.CreateConnectionDiagnostic(ctx, connectionDiagnostic); err != nil {
		return nil, trace.Wrap(err)
	}

	// Test Connection to S3 Buckets
	diag, diagErr, err := s.handleBucketsTest(ctx, connectionDiagnosticID)
	if err != nil || diagErr != nil {
		return diag, diagErr
	}

	// Test Connection to Glue Table
	diag, diagErr, err = s.handleGlueTest(ctx, connectionDiagnosticID)
	if err != nil || diagErr != nil {
		return diag, diagErr
	}

	// Test Connection to Athena
	diag, diagErr, err = s.handleAthenaTest(ctx, connectionDiagnosticID)
	if err != nil || diagErr != nil {
		return diag, diagErr
	}

	traceType := types.ConnectionDiagnosticTrace_CONNECTIVITY
	const message = "External Audit Storage draft permissions are configured correctly."
	connDiag, err := s.appendDiagnosticTrace(ctx, connectionDiagnosticID, traceType, message, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connDiag.SetMessage(types.DiagnosticMessageSuccess)
	connDiag.SetSuccess(true)

	if err := s.cfg.UserClient.UpdateConnectionDiagnostic(ctx, connDiag); err != nil {
		return nil, trace.Wrap(err)
	}

	return connDiag, nil
}

func (s ExternalAuditStorageConnectionTester) handleBucketsTest(ctx context.Context, connectionDiagnosticID string) (types.ConnectionDiagnostic, error, error) {
	client := s.cfg.UserClient.ExternalAuditStorageClient()

	if err := client.TestDraftExternalAuditStorageBuckets(ctx); err != nil {
		const message = "Failed to test connection to storage buckets."
		traceType := types.ConnectionDiagnosticTrace_CONNECTIVITY
		diag, diagErr := s.appendDiagnosticTrace(ctx, connectionDiagnosticID, traceType, message, err)
		if diagErr != nil {
			return diag, trace.Wrap(diagErr), err
		}

		return diag, nil, err
	}

	const message = "Connection to storage buckets were successful."
	traceType := types.ConnectionDiagnosticTrace_CONNECTIVITY
	diag, diagErr := s.appendDiagnosticTrace(ctx, connectionDiagnosticID, traceType, message, nil)
	return diag, trace.Wrap(diagErr), nil
}

func (s ExternalAuditStorageConnectionTester) handleGlueTest(ctx context.Context, connectionDiagnosticID string) (types.ConnectionDiagnostic, error, error) {
	client := s.cfg.UserClient.ExternalAuditStorageClient()

	if err := client.TestDraftExternalAuditStorageGlue(ctx); err != nil {
		const message = "Failed to test connection to glue table."
		traceType := types.ConnectionDiagnosticTrace_CONNECTIVITY
		diag, diagErr := s.appendDiagnosticTrace(ctx, connectionDiagnosticID, traceType, message, err)
		if diagErr != nil {
			return diag, trace.Wrap(diagErr), err
		}

		return diag, nil, err
	}

	const message = "Connection to glue table was successful."
	traceType := types.ConnectionDiagnosticTrace_CONNECTIVITY
	diag, diagErr := s.appendDiagnosticTrace(ctx, connectionDiagnosticID, traceType, message, nil)
	return diag, trace.Wrap(diagErr), nil
}

func (s ExternalAuditStorageConnectionTester) handleAthenaTest(ctx context.Context, connectionDiagnosticID string) (types.ConnectionDiagnostic, error, error) {
	client := s.cfg.UserClient.ExternalAuditStorageClient()

	if err := client.TestDraftExternalAuditStorageAthena(ctx); err != nil {
		const message = "Failed to perform athena test query."
		traceType := types.ConnectionDiagnosticTrace_CONNECTIVITY
		diag, diagErr := s.appendDiagnosticTrace(ctx, connectionDiagnosticID, traceType, message, err)
		if err != nil {
			return diag, trace.Wrap(diagErr), err
		}

		return diag, nil, err
	}

	const message = "Athena test query was successful."
	traceType := types.ConnectionDiagnosticTrace_CONNECTIVITY
	diag, diagErr := s.appendDiagnosticTrace(ctx, connectionDiagnosticID, traceType, message, nil)
	return diag, trace.Wrap(diagErr), nil
}

func (s ExternalAuditStorageConnectionTester) appendDiagnosticTrace(ctx context.Context, connectionDiagnosticID string, traceType types.ConnectionDiagnosticTrace_TraceType, message string, err error) (types.ConnectionDiagnostic, error) {
	connDiag, err := s.cfg.UserClient.AppendDiagnosticTrace(
		ctx,
		connectionDiagnosticID,
		types.NewTraceDiagnosticConnection(
			traceType,
			message,
			err,
		))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return connDiag, nil
}
