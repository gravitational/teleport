/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { App } from 'gen-proto-ts/teleport/lib/teleterm/v1/app_pb';

/**
 * Given a complete app, returns its public address mean to be used with VNet. Appends the target
 * port if given.
 */
export const appToAddrToCopy = (app: App, targetPort?: number | undefined) =>
  targetPort ? `${app.publicAddr}:${targetPort}` : app.publicAddr;
