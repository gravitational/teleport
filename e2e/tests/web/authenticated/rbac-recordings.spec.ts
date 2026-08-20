import { expect, test } from '@gravitational/e2e/helpers/test';

test.describe('session list only', () => {
  test.use({
    user: {
      roles: [{ file: '@gravitational/e2e/roles/rbac-session-list.yaml' }],
      recordings: ['ssh-recording-1'],
    },
  });

  test('verify that playing a recorded session is denied without read access', async ({
    playerPage,
    recordingIds,
  }) => {
    await playerPage.goto(recordingIds['ssh-recording-1'], 'ssh');

    await expect(
      playerPage.getByText('Session recording not found')
    ).toBeVisible();
  });
});

test.describe('session list and read', () => {
  test.use({
    user: {
      roles: [{ file: '@gravitational/e2e/roles/rbac-session-read.yaml' }],
      recordings: ['ssh-recording-1'],
    },
  });

  test('verify that a user can replay a session with read access', async ({
    playerPage,
    recordingIds,
  }) => {
    await playerPage.goto(recordingIds['ssh-recording-1'], 'ssh');

    await expect(playerPage.terminal).toBeVisible();
  });
});
