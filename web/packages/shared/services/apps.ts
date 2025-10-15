/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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
import { AppProtocol } from 'shared/services/types';

export type AwsRole = {
  name: string;
  arn: string;
  display: string;
  accountId: string;
};

/**
 * getAppProtocol returns the protocol of the application. Equivalent to
 * types.Application.GetProtocol.
 */
export function getAppProtocol(appURI: string): AppProtocol {
  if (appURI.startsWith('tcp://')) {
    return 'TCP';
  }
  if (appURI.startsWith('mcp+')) {
    return 'MCP';
  }
  return 'HTTP';
}

/**
 * getAppUriScheme extracts the scheme from the app URI.
 */
export function getAppUriScheme(appURI: string): string {
  const sepIdx = appURI.indexOf('://');
  return sepIdx > 0 ? appURI.slice(0, sepIdx) : '';
}
