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

import { getBotType } from './consts';
import { BotUiFlow } from './types';

describe('getBotType', () => {
  test('no labels', () => {
    expect(getBotType(null)).toBeNull();
  });

  test('valid github-actions-ssh label', () => {
    const labels = new Map(
      Object.entries({ 'teleport.internal/ui-flow': 'github-actions-ssh' })
    );
    expect(getBotType(labels)).toEqual(BotUiFlow.GitHubActionsSsh);
  });

  test('unknown label value', () => {
    const labels = new Map(
      Object.entries({ 'teleport.internal/ui-flow': 'unknown' })
    );
    expect(getBotType(labels)).toBeNull();
  });

  test('unrelated label', () => {
    const labels = new Map(
      Object.entries({ 'unrelated-label': 'github-actions-ssh' })
    );
    expect(getBotType(labels)).toBeNull();
  });
});
