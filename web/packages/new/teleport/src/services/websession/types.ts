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

import type { TrustedDeviceRequirement } from 'gen-proto-ts/teleport/legacy/types/trusted_device_requirement_pb';

export interface RenewSessionRequest {
  requestId?: string;
  switchback?: boolean;
  reloadUser?: boolean;
}

export interface DeleteWebSessionResponse {
  samlSloUrl?: string;
}

export interface BearerToken {
  accessToken: string;
  created: number;
  expiresIn: number;
  sessionExpires: Date;
  sessionExpiresIn: number;
  sessionInactiveTimeout: number;
  trustedDeviceRequirement?: TrustedDeviceRequirement;
}

export interface BackendBearerToken {
  token: string;
  expires_in: number;
  sessionExpires: Date;
  sessionExpiresIn: number;
  sessionInactiveTimeout: number;
  trustedDeviceRequirement?: number;
}
