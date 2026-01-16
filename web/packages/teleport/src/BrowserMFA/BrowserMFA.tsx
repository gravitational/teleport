import { useEffect } from 'react';
import { useParams } from 'react-router';

import { Alert, Flex, H1, Indicator } from 'design';
import CardError, { AccessDenied } from 'design/CardError';
import useAttempt from 'shared/hooks/useAttemptNext';
import { isAbortError } from 'shared/utils/error';

import AuthnDialog from 'teleport/components/AuthnDialog';
import { useMfa } from 'teleport/lib/useMfa';
import auth from 'teleport/services/auth';
import { MfaChallengeScope } from 'teleport/services/auth/auth';

// import history from 'teleport/services/history/history';

export function BrowserMFA() {
  const { requestId } = useParams<{ requestId: string }>();
  const { attempt, setAttempt } = useAttempt('processing');

  // This route always requires MFA.
  const mfa = useMfa({
    req: {
      // TODO: Add scope?
      scope: MfaChallengeScope.USER_SESSION,
    },
    isMfaRequired: true,
  });

  useEffect(() => {
    const signal = new AbortController();

    async function promptWebauthnAndRedirect() {
      try {
        let resp = await auth.browserMFA(mfa, requestId);
        console.log(resp);
        // history.push(resp, true);
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
  }, [requestId]); // eslint-disable-line react-hooks/exhaustive-deps -- Only run the effect once on mount

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
