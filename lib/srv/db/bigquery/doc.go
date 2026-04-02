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

// Package bigquery implements the BigQuery database protocol.
//
// BigQuery is a serverless data warehouse on Google Cloud Platform.
// This engine proxies BigQuery API requests and provides query-level
// audit logging for all SQL queries.
//
// # Connection Flow
//
//  1. A Teleport database agent starts and registers itself with the auth server
//     with a BigQuery resource.
//  2. A user runs `tsh db connect <bigquery-db>` which starts a local proxy.
//  3. The local proxy tunnels requests through the Teleport proxy to the
//     database agent.
//  4. The database agent receives the request, performs RBAC checks, and
//     forwards the request to the BigQuery API with appropriate credentials.
//  5. All queries are logged to the audit log.
//
// # Authentication
//
// BigQuery uses OAuth2 access tokens for authentication. The database agent
// obtains tokens by impersonating a GCP service account configured in the
// database resource.
package bigquery
