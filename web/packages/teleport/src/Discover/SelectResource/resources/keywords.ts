/**
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

export const baseServerKeywords = ['server', 'node', 'ssh'];
export const awsKeywords = ['aws', 'amazon', 'amazon web services'];
export const kubeKeywords = ['kubernetes', 'k8s', 'kubes', 'cluster'];
export const selfHostedKeywords = ['self hosted', 'self-hosted'];

export const baseDatabaseKeywords = ['db', 'database', 'databases'];
export const awsDatabaseKeywords = [...baseDatabaseKeywords, ...awsKeywords];
export const gcpKeywords = [
  ...baseDatabaseKeywords,
  'gcp',
  'google cloud platform',
];
export const selfHostedDatabaseKeywords = [
  ...baseDatabaseKeywords,
  ...selfHostedKeywords,
];
export const azureKeywords = [...baseDatabaseKeywords, 'microsoft azure'];
