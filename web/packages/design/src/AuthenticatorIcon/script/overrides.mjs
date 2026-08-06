/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

// Vendor icons we substitute for the ones shipped in the upstream dataset, keyed by a pattern tested
// against the raw dataset name. A vendor typically registers dozens of AAGUIDs sharing one mark, so
// matching on the name replaces all of them at once and keeps covering AAGUIDs added upstream later.
//
// `icon` supplies both themes; `light` and `dark` override a single theme when a mark needs separate
// artwork. Paths are relative to script/overrides/, and only PNG and SVG are supported.
//
// An override that matches nothing fails the generator rather than silently doing nothing.
export const iconOverrides = [
  {
    // The dataset ships the Yubico mark in two encodings: a transparent SVG, and a 32px PNG on an
    // opaque white background that renders as a white box on the dark theme. yubico.svg is a verbatim
    // copy of the former, so every Yubico AAGUID gets the scalable transparent one.
    id: 'yubico',
    match: /yubikey|by yubico/i,
    icon: 'yubico.svg',
  },
];
