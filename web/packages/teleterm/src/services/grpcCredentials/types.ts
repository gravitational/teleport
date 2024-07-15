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

// Each process creates its own key pair. The public key is saved to disk under the specified
// filename, the private key stays in the memory.
//
// `Renderer`, `Tshd`, and `MainProcess` file names are also used on the tshd side.
export enum GrpcCertName {
  Renderer = 'renderer.crt',
  Tshd = 'tshd.crt',
  Shared = 'shared.crt',
  MainProcess = 'main-process.crt',
}
