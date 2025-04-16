#!/usr/bin/env node
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
import { rm } from 'node:fs/promises';
import { join } from 'node:path';

// wasm-pack build artifacts were originally kept in 'teleport' before being moved to 'shared'.
// While they don’t cause significant issues, they show up in autocomplete suggestions,
// so it's best to clean them up.
// TODO(gzdunek) DELETE IN v20.0.0
await rm(join(import.meta.dirname, '..', 'packages/teleport/src/ironrdp'), {
  recursive: true,
  force: true,
});
