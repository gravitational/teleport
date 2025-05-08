import { Page } from '@playwright/test';

// mockWebAuthn sets up a virtual webauthn authenticator on the page.
export async function mockWebAuthn(page: Page) {
  const cdpSession = await page.context().newCDPSession(page);
  await cdpSession.send('WebAuthn.enable');

  const { authenticatorId } = await cdpSession.send(
    'WebAuthn.addVirtualAuthenticator',
    {
      options: {
        protocol: 'ctap2',
        transport: 'internal',
        hasResidentKey: true,
        hasUserVerification: true,
        isUserVerified: true,
      },
    }
  );

  const cleanup = async () => {
    await cdpSession.send('WebAuthn.removeVirtualAuthenticator', {
      authenticatorId,
    });
    await cdpSession.detach();
  };

  return { authenticatorId, cdpSession, cleanup };
}
