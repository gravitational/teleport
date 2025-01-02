import { parsePath } from 'history';
import { useEffect } from 'react';

import { Flex, Indicator } from 'design';
import { AccessDenied } from 'design/CardError';
import useAttempt from 'shared/hooks/useAttemptNext';
import { isAbortError } from 'shared/utils/abortError';
import { bufferToBase64url } from 'shared/utils/base64';

import auth, { MfaChallengeScope } from 'teleport/services/auth/auth';
import history from 'teleport/services/history';

export function SAMLIdPLogin() {
  const { attempt, setAttempt } = useAttempt('processing');

  useEffect(() => {
    const signal = new AbortController();

    async function promptWebauthnAndRedirect() {
      try {
        // Prompt for MFA, we only get routed here when MFA
        // is required for SAML IdP Sessions.
        const webauthnResponse = await auth.getWebauthnResponse(
          MfaChallengeScope.USER_SESSION,
          false,
          null,
          signal.signal
        );
        // url safe base64 encoding is chosen here because with just a
        // plain JSON or even encodeURIComponent encoded string, it can break
        // the CSP header or result in a mismatch between the CSP directive and
        // form action URL when the value passes through the Go's html templating.
        const mfaResponseBytes = new TextEncoder().encode(
          JSON.stringify({
            webauthnAssertionResponse: webauthnResponse,
          })
        );
        const urlSafeMfaResponse = bufferToBase64url(mfaResponseBytes.buffer);

        // Add the mfa response as a query param while preserving
        // existing query params (saml request).
        let { pathname, search } = parsePath(history.getRedirectParam());
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
