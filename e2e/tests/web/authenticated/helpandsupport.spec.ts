import { env } from 'node:process';

import { expect, test } from '@gravitational/e2e/helpers/test';

test.describe('help and support page', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.getByRole('button', { name: 'User Menu' }).click();
    await page.getByRole('link', { name: 'Help & Support' }).click();
  });

  test('contains support and resource links', async ({ page }) => {
    const communityPromise = page.waitForEvent('popup');
    await page
      .getByRole('link', { name: 'Ask the Community Questions' })
      .click();
    const communityPage = await communityPromise;
    await expect(communityPage).toHaveURL(
      'https://github.com/gravitational/teleport/discussions'
    );
    await communityPage.close();

    // The new feature link leads to a GitHub URL that requires login. To avoid
    // breaking the test if GitHub subtly changes the behavior here, fall back to
    // checking the href attribute and checking actionability.
    const newFeatureLink = await page.getByRole('link', {
      name: 'Request a New Feature',
    });
    await expect(newFeatureLink).toHaveAttribute(
      'href',
      'https://github.com/gravitational/teleport/issues/new/choose'
    );
    await newFeatureLink.click({ trial: true });

    // Playwright doesn't allow capturing an e-mail third-party tool launch. Fall
    // back to checking the href attribute and checking actionability.
    const feedbackLink = await page.getByRole('link', {
      name: 'Send Product Feedback',
    });
    await expect(feedbackLink).toHaveAttribute(
      'href',
      'mailto:support@goteleport.com'
    );
    await feedbackLink.click({ trial: true });

    const gettingStartedPromise = page.waitForEvent('popup');
    await page.getByRole('link', { name: 'Get Started Guide' }).click();
    const gettingStartedPage = await gettingStartedPromise;
    await expect(
      gettingStartedPage.getByRole('heading', { level: 1 })
    ).toContainText('Get Started with Teleport');
    await gettingStartedPage.close();

    const tshGuidePromise = page.waitForEvent('popup');
    await page.getByRole('link', { name: 'tsh User Guide' }).click();
    const tshGuidePage = await tshGuidePromise;
    await expect(tshGuidePage.getByRole('heading', { level: 1 })).toContainText(
      'Using the tsh Command Line Tool'
    );
    await tshGuidePage.close();

    const adminGuidesPromise = page.waitForEvent('popup');
    await page.getByRole('link', { name: 'Admin Guides' }).click();
    const adminGuidesPage = await adminGuidesPromise;
    await expect(
      adminGuidesPage.getByRole('heading', { level: 1 })
    ).toContainText('Cluster Management');
    await adminGuidesPage.close();

    const troubleshootingPromise = page.waitForEvent('popup');
    await page.getByRole('link', { name: 'Troubleshooting Guide' }).click();
    const troubleshootingPage = await troubleshootingPromise;
    await expect(
      troubleshootingPage.getByRole('heading', { level: 1 })
    ).toContainText('Troubleshooting');
    await troubleshootingPage.close();

    const downloadsPromise = page.waitForEvent('popup');
    await page.getByRole('link', { name: 'Download Page' }).click();
    const downloadsPage = await downloadsPromise;
    await expect(
      downloadsPage.getByRole('heading', { level: 1 })
    ).toContainText('Download and Deployment Options');
    await downloadsPage.close();

    const faqPromise = page.waitForEvent('popup');
    await page.getByRole('link', { name: 'FAQ' }).click();
    const faqPage = await faqPromise;
    await expect(faqPage.getByRole('heading', { level: 1 })).toContainText(
      'Teleport FAQ'
    );
    await faqPage.close();

    const changelogPromise = page.waitForEvent('popup');
    await page.getByRole('link', { name: 'Product Changelog' }).click();
    const changelogPage = await changelogPromise;
    await expect(
      changelogPage.getByRole('heading', { level: 1 })
    ).toContainText('Teleport Changelog');
    await changelogPage.close();

    const upcomingReleasesPromise = page.waitForEvent('popup');
    await page.getByRole('link', { name: 'Upcoming Releases' }).click();
    const upcomingReleasesPage = await upcomingReleasesPromise;
    await expect(
      upcomingReleasesPage.getByRole('heading', { level: 1 })
    ).toContainText('Teleport Upcoming Releases');
    await upcomingReleasesPage.close();

    const blogPromise = page.waitForEvent('popup');
    await page.getByRole('link', { name: 'Teleport Blog' }).click();
    const blogPage = await blogPromise;
    await expect(blogPage.getByRole('heading', { level: 1 })).toContainText(
      'Teleport Blog'
    );
    await blogPage.close();
  });

  test('contains cluster information', async ({ page }) => {
    const clusterInformationSection = page.getByRole('region', {
      name: 'Cluster Information',
    });
    await expect(clusterInformationSection).toContainText(
      'Cluster Name: teleport-e2e'
    );
    await expect(clusterInformationSection).toContainText(
      `Teleport Version: ${env['E2E_TELEPORT_VERSION']}`
    );
    const host = URL.parse(page.url())?.host;
    await expect(clusterInformationSection).toContainText(
      `Public Address: ${host}`
    );
  });

  test('button to unlock premium support leads to the sales page', async ({
    page,
  }) => {
    const salesPromise = page.waitForEvent('popup');
    await page
      .getByRole('link', { name: 'Unlock Premium Support with Enterprise' })
      .click();
    const salesPage = await salesPromise;
    await expect(salesPage.getByRole('heading')).toContainText(
      'Schedule a sales conversation'
    );
    await salesPage.close();
  });
});
