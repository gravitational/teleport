import { login } from '@gravitational/e2e/helpers/login';
import { test } from '@gravitational/e2e/helpers/test';

test.use({ user: { roles: ['access', 'editor'] } });

test('verify that a user can log in with username, password, and webauthn', async ({
  page,
  username,
}) => {
  await login(page, username);
});
