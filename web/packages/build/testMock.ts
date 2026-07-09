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

/**
 * Runner-agnostic mock API for the jest -> vitest migration: Jest's `jest` under
 * Jest, Vitest's `vi` under Vitest. Shared helpers that run under both use this
 * rather than importing `vitest` (which breaks Jest) or a bare `jest`/`vi` (only
 * one exists at a time). Hoisted `vi.mock`/`jest.mock` can't go through it.
 */

// jest/vi are runner-injected scoped globals (Jest doesn't put `jest` on
// globalThis); the declarations shadow them so this type-checks under either.
declare const jest: unknown;
declare const vi: unknown;

export const mockFn = (
  typeof jest !== 'undefined' ? jest : vi
) as (typeof import('vitest'))['vi'];
