import { expect, test } from '@gravitational/e-e2e/helpers/test';

test.use({
  recordings: ['ssh-recording-1'],
});

test('session summary sidebar renders for a seeded recording', async ({
  playerPage,
  recordingIds,
}) => {
  await playerPage.goto(recordingIds['ssh-recording-1'], 'ssh');

  await playerPage.expectSummaryVisible();
  await playerPage.expectRiskLevel('High');

  await expect(playerPage.shortDescription).toContainText('/etc/hosts');

  await expect(playerPage.timelineEvent(/ping google\.com/)).toBeVisible();
  await expect(playerPage.timelineEvent(/Edited \/etc\/hosts/)).toBeVisible();

  await playerPage.openDetailedDescription();
  await expect(playerPage.detailedDescription).toContainText(
    'SSH Session Summary'
  );
});
