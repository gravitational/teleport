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

/**
 * An env var which controls whether tsh is going to download an up-to-date version of itself
 * to ~/.tsh/bin and re-execute itself. In Connect, we always want it to be set to 'off', as Connect
 * needs to use the bundled tsh where the version of tsh matches exactly the version of Connect.
 *
 * See RFD 144 for more details.
 */
export const TSH_AUTOUPDATE_ENV_VAR = 'TELEPORT_TOOLS_VERSION';
export const TSH_AUTOUPDATE_OFF = 'off';
