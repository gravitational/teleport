import { render, screen, waitFor } from 'design/utils/testing';

import cfg from 'teleport/config';
import history from 'teleport/services/history';
import {
  SsoChallengeResponse,
  WebauthnAssertionResponse,
} from 'teleport/services/mfa';

import { SAMLIdPLogin } from './SAMLIdPLogin';

const baseUrl = 'http://example.com';

// Use a shared mock function for getChallengeResponse so spies and resolved values
// are consistent across all uses of useMfa in the component and tests.
const mockGetChallengeResponse = jest.fn();

// Mock useMfa to always return the same mockGetChallengeResponse instance.
jest.mock('teleport/lib/useMfa', () => {
  return {
    useMfa: () => ({
      getChallengeResponse: mockGetChallengeResponse,
    }),
    shouldShowMfaPrompt: jest.fn(),
  };
});

describe('valid redirect_uri', () => {
  describe('with WebAuthn MFA', () => {
    beforeEach(() => {
      jest.clearAllMocks();
      jest.spyOn(history, 'push').mockImplementation();
      mockGetChallengeResponse.mockResolvedValue({
        webauthn_response: {} as WebauthnAssertionResponse,
      });
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
        expect(history.push).toHaveBeenCalledWith(
          expectedRedirection +
            '?MFAResponse=eyJ3ZWJhdXRobkFzc2VydGlvblJlc3BvbnNlIjp7fSwibWZhX3Jlc3BvbnNlIjp7fX0' + // Base64 encoded JSON: `{"mfa_response":{}}`
            '&Webauthn=eyJ3ZWJhdXRobkFzc2VydGlvblJlc3BvbnNlIjp7fSwibWZhX3Jlc3BvbnNlIjp7fX0', // Base64 encoded JSON: `{"webauthnAssertionResponse":{}}`
          true
        );
      });
    });
  });

  describe('with SSO MFA', () => {
    beforeEach(() => {
      jest.clearAllMocks();
      jest.spyOn(history, 'push').mockImplementation();
      mockGetChallengeResponse.mockResolvedValue({
        sso_response: {} as SsoChallengeResponse,
      });
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
        expect(history.push).toHaveBeenCalledWith(
          expectedRedirection + '?MFAResponse=eyJtZmFfcmVzcG9uc2UiOnt9fQ', // Base64 encoded JSON: `{"mfa_response":{}}`
          true
        );
      });
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
