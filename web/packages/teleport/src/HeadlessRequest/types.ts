/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

export enum HeadlessAuthenticationType {
  UNSPECIFIED = 0,
  HEADLESS = 1,
  BROWSER = 2,
  SESSION = 3,
}

export function getHeadlessAuthTypeLabel(type: number): string {
  switch (type) {
    case HeadlessAuthenticationType.HEADLESS:
      return 'headless';
    case HeadlessAuthenticationType.BROWSER:
      return 'browser';
    case HeadlessAuthenticationType.SESSION:
      return 'session';
    default:
      return 'unknown';
  }
}

export function getActionFromAuthType(authType: HeadlessAuthenticationType): string {
  switch (authType) {
    case HeadlessAuthenticationType.BROWSER:
      return 'login';
    case HeadlessAuthenticationType.HEADLESS:
    case HeadlessAuthenticationType.SESSION:
      return 'command';
    default:
      return 'unknown';
  }
}
