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

// LockKind is the expected backend value
// to define the type of resource to be locked.
export type LockKind =
  | 'user'
  | 'role'
  | 'login'
  | 'node'
  | 'mfa_device'
  | 'windows_desktop'
  | 'access_request'
  | 'device'; // trusted devices

export type Lock = {
  name: string;
  message: string;
  expires: string;
  createdAt: string;
  createdBy: string;
  targetLookup: Partial<Record<LockKind, string>>;
  targets: LockTarget[];
};

export type LockTarget = {
  kind: LockKind;
  name: string;
};

export type CreateLockRequest = {
  targets: Partial<Record<LockKind, string>>;
  message: string;
  ttl: string;
};

export type ApiLock = {
  name: string;
  message?: string;
  expires?: string;
  createdAt?: string;
  createdBy?: string;
  targets: Partial<{
    user: string;
    role: string;
    login: string;
    node: string;
    mfa_device: string;
    windows_desktop: string;
    device: string;
    access_request: string;
  }>;
};
