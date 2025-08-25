import { parsePath } from 'history';
import { useEffect } from 'react';

import { Alert, Flex, H1, Indicator } from 'design';
import CardError, { AccessDenied } from 'design/CardError';
import useAttempt from 'shared/hooks/useAttemptNext';
import { bufferToBase64url } from 'shared/utils/base64';
import { isAbortError } from 'shared/utils/error';

import cfg from 'e-teleport/config';
import AuthnDialog from 'teleport/components/AuthnDialog';
import { useMfa } from 'teleport/lib/useMfa';
import { MfaChallengeScope } from 'teleport/services/auth/auth';
import history from 'teleport/services/history';

export function SAMLIdPLogin() {
  const { attempt, setAttempt } = useAttempt('processing');

  // This route always requires MFA.
  const mfa = useMfa({
    req: {
      scope: MfaChallengeScope.USER_SESSION,
    },
    isMfaRequired: true,
  });

  useEffect(() => {
    const signal = new AbortController();

    async function promptWebauthnAndRedirect() {
      try {
        if (!isValidRedirectUri()) {
          setAttempt({
            status: 'failed',
            statusText: 'Invalid redirect URI',
            statusCode: 400,
          });
          return;
        }

        const mfaResponse = await mfa.getChallengeResponse();

        // Short circuit if no supported MFA methods were completed.
        if (!mfaResponse.webauthn_response && !mfaResponse.sso_response) {
          throw new Error(
            "Multi-factor authentication (MFA) is required to access this resource but the current user has no supported MFA devices enrolled; see Account Settings in the Web UI or use 'tsh mfa add' to register an MFA device"
          );
        }

        // Use URL-safe base64 encoding to avoid CSP issues with JSON or encodeURIComponent.
        const mfaResponseBytes = new TextEncoder().encode(
          JSON.stringify({
            // Keep webauthnAssertionResponse for backward compatibility. If
            // webauthn_response is not present, webauthnAssertionResponse will
            // not be included in the final payload.
            // TODO(cthach): DELETE IN v20.0.0.
            webauthnAssertionResponse:
              mfaResponse.webauthn_response ?? undefined,

            mfa_response:
              mfaResponse.webauthn_response || mfaResponse.sso_response,
          })
        );
        const urlSafeMfaResponse = bufferToBase64url(mfaResponseBytes.buffer);

        // Add the mfa response as a query param while preserving
        // existing query params (saml request).
        // TODO(sshah): replace getRedirectParam() with getEntryRoute()
        // that ensures base and route URL once the URL validation patch
        // is published in the private release.
        let entryUrl = history.getRedirectParam();
        entryUrl = history.ensureKnownRoute(entryUrl);
        let { pathname, search } = parsePath(history.ensureBaseUrl(entryUrl));

        // Query parameters are defined in e/lib/idp/saml/response.go.
        const searchParams = new URLSearchParams(search);

        // Set MFAResponse parameter to the base64-encoded JSON of the MFA response.
        searchParams.set('MFAResponse', urlSafeMfaResponse);

        // Set Webauthn parameter for backward compatibility.
        // TODO(cthach): DELETE IN v20.0.0.
        if (mfaResponse.webauthn_response) {
          searchParams.set('Webauthn', urlSafeMfaResponse);
        }

        const url = new URL(pathname, window.location.origin);
        url.search = searchParams.toString();

        // Redirect to SAML IdP login handler. The handler will verify the MFA
        // response and redirect to the original SAML IdP URL with the SAML
        // assertion.
        history.push(url.toString(), true);
      } catch (err) {
        // ignore abort errors
        if (isAbortError(err)) {
          return;
        }

        setAttempt({
          status: 'failed',
          statusText: err.message,
        });
      }
    }

    promptWebauthnAndRedirect();

    return () => {
      signal.abort();
    };
  }, []); // eslint-disable-line react-hooks/exhaustive-deps -- Only run the effect once on mount

  if (attempt.status === 'failed') {
    if (attempt.statusCode === 400) {
      return <BadRequest message={attempt.statusText} />;
    }
    return <SAMLLoginAccessDenied statusText={attempt.statusText} />;
  }

  return (
    <>
      <SAMLLoginProcessing />
      <AuthnDialog mfaState={mfa} />
    </>
  );
}

export function SAMLLoginProcessing() {
  return (
    <Flex height="180px" justifyContent="center" alignItems="center" flex="1">
      <Indicator />
    </Flex>
  );
}

interface SAMLLoginAccessDeniedProps {
  statusText: string;
}

export function SAMLLoginAccessDenied(props: SAMLLoginAccessDeniedProps) {
  return <AccessDenied message={props.statusText} />;
}

/**
 * isValidRedirectUri checks if the origin in the redirect_uri
 * param matches with the baseUrl, which is an origin value of the
 * URL in the current active browser tab.
 */
function isValidRedirectUri(): boolean {
  const redirectUri = history.getRedirectParam();
  try {
    const parsedRedirectUri = new URL(redirectUri);
    if (parsedRedirectUri.origin === cfg.oss.baseUrl) {
      return true;
    }
  } catch {
    return false;
  }

  return false;
}

// TODO(sshah): move this component to CardError.jsx once
// the URL validation patch is published in the private release.
export const BadRequest = ({ message = '' }) => (
  <CardError>
    <H1 mb={4} textAlign="center">
      400 Bad Request
    </H1>
    <Alert mt={2} mb={4}>
      {message}
    </Alert>
  </CardError>
);
