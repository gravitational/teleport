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

import { BrowserMfaAccessDenied, BrowserMfaProcessing } from './BrowserMFA';

export default {
  title: 'Teleport/BrowserMFA',
};

export function Processing() {
  return <BrowserMfaProcessing />;
}

export function AccessDenied() {
  return <BrowserMfaAccessDenied statusText="MFA validation failed" />;
}

export function AccessDeniedWithLongMessage() {
  return (
    <BrowserMfaAccessDenied statusText="Your browser could not complete this authentication request. Please retry and ensure your security key or passkey is available." />
  );
}
