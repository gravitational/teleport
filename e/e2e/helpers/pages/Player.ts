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
