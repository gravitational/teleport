import { parsePath } from 'history';
import { useEffect } from 'react';

import { Alert, Flex, H1, Indicator } from 'design';
import CardError, { AccessDenied } from 'design/CardError';
import useAttempt from 'shared/hooks/useAttemptNext';
import { isAbortError } from 'shared/utils/abortError';
import { bufferToBase64url } from 'shared/utils/base64';

import cfg from 'e-teleport/config';
import auth, { MfaChallengeScope } from 'teleport/services/auth/auth';
import history from 'teleport/services/history';

export function SAMLIdPLogin() {
  const { attempt, setAttempt } = useAttempt('processing');

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
        // Prompt for MFA, we only get routed here when MFA
        // is required for SAML IdP Sessions.
        const mfaChallenge = await auth.getMfaChallenge(
          { scope: MfaChallengeScope.USER_SESSION },
          signal.signal
        );

        const mfaResponse = await auth.getMfaChallengeResponse(
          mfaChallenge,
          'webauthn'
        );
        // url safe base64 encoding is chosen here because with just a
        // plain JSON or even encodeURIComponent encoded string, it can break
        // the CSP header or result in a mismatch between the CSP directive and
        // form action URL when the value passes through the Go's html templating.
        const mfaResponseBytes = new TextEncoder().encode(
          JSON.stringify({
            // TODO(Joerger): Handle non-webauthn response.
            webauthnAssertionResponse: mfaResponse.webauthn_response,
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
        if (search) {
          search = `${search}&Webauthn=${urlSafeMfaResponse}`;
        } else {
          search = `?Webauthn=${urlSafeMfaResponse}`;
        }

        const url = `${pathname}${search}`;
        history.push(url, true);
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
  }, []);

  if (attempt.status === 'failed') {
    if (attempt.statusCode === 400) {
      return <BadRequest message={attempt.statusText} />;
    }
    return <SAMLLoginAccessDenied statusText={attempt.statusText} />;
  }

  return <SAMLLoginProcessing />;
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
