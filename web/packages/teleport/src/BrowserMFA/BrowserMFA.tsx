import { useEffect } from 'react';
import { useParams } from 'react-router';

import { Alert, Flex, H1, Indicator } from 'design';
import CardError, { AccessDenied } from 'design/CardError';
import useAttempt from 'shared/hooks/useAttemptNext';
import { isAbortError } from 'shared/utils/error';

import AuthnDialog from 'teleport/components/AuthnDialog';
import { useMfa } from 'teleport/lib/useMfa';
import auth from 'teleport/services/auth';

export function BrowserMFA() {
  const { requestId } = useParams<{ requestId: string }>();
  const { attempt, setAttempt } = useAttempt('processing');

  const mfa = useMfa({
    isMfaRequired: true,
    req: {
      browserMfaRequestId: requestId,
    },
  });

  useEffect(() => {
    async function promptWebauthnAndRedirect() {
      try {
        if (!requestId) {
          setAttempt({
            status: 'failed',
            statusText: 'Missing request ID',
            statusCode: 400,
          });
          return;
        }

        const resp = await auth.browserMFA(mfa, requestId);
        window.location.href = resp;
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
  }, [requestId]);

  if (attempt.status === 'failed') {
    if (attempt.statusCode === 400) {
      return <BadRequest message={attempt.statusText} />;
    }
    return <BrowserMFAAccessDenied statusText={attempt.statusText} />;
  }

  return (
    <>
      <BrowserMFAProcessing />
      <AuthnDialog mfaState={mfa} />
    </>
  );
}

export function BrowserMFAProcessing() {
  return (
    <Flex height="180px" justifyContent="center" alignItems="center" flex="1">
      <Indicator />
    </Flex>
  );
}

interface BrowserMFAAccessDeniedProps {
  statusText: string;
}

export function BrowserMFAAccessDenied(props: BrowserMFAAccessDeniedProps) {
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