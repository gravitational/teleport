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

import type { Plugin } from 'vite';

/*
  @xterm/addon-image references the bare `WebAssembly` identifier at module scope (via its bundled InWasm helper).
  In environments where WebAssembly is not defined this throws a ReferenceError on import,
  crashing the app. We replace all references with a local shim that either delegates to
  the real WebAssembly or throws a descriptive error, letting us statically import the
  module (no extra chunk) while deferring failure to addon construction time.
*/

const shim = `
const __WASM__ = typeof WebAssembly !== 'undefined' ? WebAssembly : {
  validate: () => false,
  compile() { throw new Error('WebAssembly is not available'); },
  instantiate() { throw new Error('WebAssembly is not available'); },
  Module: class { constructor() { throw new Error('WebAssembly is not available'); } },
  Instance: class { constructor() { throw new Error('WebAssembly is not available'); } },
  Memory: class { constructor() { throw new Error('WebAssembly is not available'); } },
};
`;

export function guardWasmPlugin(): Plugin {
  return {
    name: 'guard-wasm',
    transform(code, id) {
      if (id.includes('@xterm/addon-image')) {
        return shim + '\n' + code.replace(/\bWebAssembly\b/g, '__WASM__');
      }
    },
  };
}
