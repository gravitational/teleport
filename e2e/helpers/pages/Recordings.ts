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

import type { Page } from '@playwright/test';

import { CLUSTER_NAME } from '../test';

import { PlayerPage } from './Player';

export class RecordingsPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto(`/web/cluster/${CLUSTER_NAME}/recordings`);
  }

  /**
   * Opens the first recording in a new popup tab and returns a PlayerPage.
   */
  async openFirstRecording() {
    const recordingLink = this.page.getByTestId('recording-item').first();

    await recordingLink.waitFor({ state: 'visible' });

    const popupPromise = this.page.waitForEvent('popup');

    await recordingLink.click();

    const popup = await popupPromise;

    await popup.waitForLoadState('load');

    return new PlayerPage(popup);
  }
}
