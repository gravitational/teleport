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

import { expect, type Page } from '@playwright/test';

import { CLUSTER_NAME } from '../test';

export type RecordingType = 'ssh' | 'k8s' | 'desktop' | 'database';

export class PlayerPage {
  constructor(private page: Page) {}

  async goto(sessionId: string, recordingType: RecordingType) {
    await this.page.goto(
      `/web/cluster/${CLUSTER_NAME}/session/${sessionId}?recordingType=${recordingType}&durationMs=1000`
    );
  }

  async expectError(text: string | RegExp) {
    await expect(this.page.getByText(text)).toBeVisible();
  }
}
