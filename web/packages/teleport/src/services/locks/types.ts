/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
