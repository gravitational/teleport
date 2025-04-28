import { render, screen, waitFor } from 'design/utils/testing';

import cfg from 'teleport/config';
import auth from 'teleport/services/auth/auth';
import history from 'teleport/services/history';

import { SAMLIdPLogin } from './SAMLIdPLogin';

const baseUrl = 'http://example.com';

describe('valid redirect_uri', () => {
  beforeEach(() => {
    jest.spyOn(history, 'push').mockImplementation();
    jest.spyOn(auth, 'getMfaChallenge').mockResolvedValueOnce({
      totpChallenge: true,
      webauthnPublicKey: {} as PublicKeyCredentialRequestOptions,
    });
    jest.spyOn(auth, 'getMfaChallengeResponse').mockResolvedValueOnce({});
  });

  cfg.baseUrl = baseUrl;
  const tests: Array<{
    name: string;
    redirectUri: string;
    expectedRedirection: string;
  }> = [
    {
      name: 'valid URL',
      redirectUri: 'http://example.com' + cfg.routes.samlIdpSso,
      expectedRedirection: 'http://example.com' + cfg.routes.samlIdpSso,
    },
    {
      name: 'valid host, invalid path',
      redirectUri: 'http://example.com/enterprise/test/saml-idp/sso',
      expectedRedirection: 'http://example.com/web',
    },
  ];

  test.each(tests)('$name', async ({ redirectUri, expectedRedirection }) => {
    jest
      .spyOn(history, 'getRedirectParam')
      .mockReturnValue(redirectUri.toString());
    render(<SAMLIdPLogin />);

    await waitFor(() => {
      // Webauthn value "e30" represents an empty JSON tag {} converted to base64 (e30=).
      // The trailing '=' character is removed by the bufferToBase64url converter.
      expect(history.push).toHaveBeenCalledWith(
        expectedRedirection + '?Webauthn=e30',
        true
      );
    });
  });
});

describe('invalid redirect_uri', () => {
  cfg.baseUrl = baseUrl;
  const tests: Array<{
    name: string;
    redirectUri: string;
  }> = [
    {
      name: 'path only',
      redirectUri: cfg.routes.samlIdpSso,
    },
    {
      name: 'malformed path',
      redirectUri: cfg.routes.samlIdpSso + 'http://attacker.com',
    },
    {
      name: 'different host',
      redirectUri: 'http://attacker.com' + cfg.routes.samlIdpSso,
    },
    {
      name: 'malformed host',
      redirectUri: 'http://example.//attacker.com' + cfg.routes.samlIdpSso,
    },
  ];

  test.each(tests)('$name', async ({ redirectUri }) => {
    jest
      .spyOn(history, 'getRedirectParam')
      .mockReturnValue(redirectUri.toString());
    render(<SAMLIdPLogin />);

    expect(
      await screen.findByText('Bad Request', { exact: false })
    ).toBeInTheDocument();
  });
});
