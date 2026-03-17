/*
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

import ace from 'ace-builds/src-min-noconflict/ace';

// Ace extension and mode files (e.g. mode-json.js, ext-searchbox.js) reference
// `ace` as a bare global identifier (`ace.define(...)`). The ace.js IIFE tries
// to set `window.ace` via `(function(){ return this })()`, but Vite 8's bundler
// (Rolldown) can drop or scope that assignment. Explicitly assign it here so
// that subsequently-evaluated extension modules can resolve the bare `ace` ref.
globalThis.ace = ace;

export default ace;
