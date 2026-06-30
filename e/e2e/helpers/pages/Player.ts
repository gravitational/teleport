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

import { PlayerPage } from '@gravitational/e2e/helpers/pages/Player';
import {
  expect,
  type Locator,
  type Page,
} from '@gravitational/e2e/helpers/test';

export type SummaryRiskLevel = 'Low' | 'Medium' | 'High' | 'Critical' | 'None';

export class PlayerPageE extends PlayerPage {
  readonly summarySection: Locator;
  readonly summaryHeading: Locator;
  readonly shortDescription: Locator;
  readonly showMoreButton: Locator;
  readonly detailedDescription: Locator;
  readonly riskLevel: Locator;

  constructor(page: Page) {
    super(page);

    this.summarySection = page.getByTestId('session-summary');
    this.summaryHeading = this.summarySection.getByRole('heading', {
      name: 'Session Summary',
      level: 3,
    });
    this.shortDescription = page.getByTestId(
      'session-summary-short-description'
    );
    this.showMoreButton = this.summarySection.getByRole('button', {
      name: 'Show more',
    });
    this.detailedDescription = page.getByTestId(
      'session-summary-detailed-description'
    );

    this.riskLevel = page.getByTestId('session-risk-level');
  }

  async expectSummaryVisible() {
    await expect(this.summaryHeading).toBeVisible();
  }

  async expectRiskLevel(level: SummaryRiskLevel) {
    await expect(this.riskLevel).toHaveText(level);
  }

  async openDetailedDescription() {
    await this.showMoreButton.click();
    await expect(this.detailedDescription).toBeVisible();
  }

  timelineEvent(title: string | RegExp): Locator {
    return this.summarySection.getByText(title);
  }
}
