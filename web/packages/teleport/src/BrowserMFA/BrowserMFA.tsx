/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { useEffect } from 'react';
import { useParams } from 'react-router';

import { Flex, Indicator } from 'design';
import { AccessDenied, BadRequest } from 'design/CardError';
import useAttempt from 'shared/hooks/useAttemptNext';

import AuthnDialog from 'teleport/components/AuthnDialog';
import { useMfa, shouldShowMfaPrompt } from 'teleport/lib/useMfa';
import auth from 'teleport/services/auth';
import { MFA_OPTION_WEBAUTHN } from 'teleport/services/mfa';

import { validateClientRedirect } from './urlValidation';

interface BrowserMFAProps {
  // onRedirect is used for testing only.
  onRedirect?: (url: string) => void;
}

export function BrowserMFA({ onRedirect = redirectTo }: BrowserMFAProps) {
  const { requestId } = useParams<{ requestId: string }>();
  const { attempt, setAttempt } = useAttempt('processing');

  const mfa = useMfa({
    isMfaRequired: true,
    req: {
      browserMfaRequestId: requestId,
    },
  });
  
  // Don't allow users to answer the webauthn challenge with an sso challenge.
  // Server side checks ensure that a webauthn device will be present.
  if (mfa.challenge) {
    mfa.challenge = { ...mfa.challenge, ssoChallenge: null };
  }

  // Auto-submit webauthn when the challenge first appears. If it fails, the
  // user is shown the AuthnDialog to retry.
  useEffect(() => {
    if (mfa.challenge?.webauthnPublicKey && mfa.attempt.status === 'processing') {
      mfa.submit('webauthn');
    }
  }, [mfa.challenge]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    const abortController = new AbortController();

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

        // Get the tsh redirect URL
        const tshRedirectUrl = await auth.browserMfaPut(
          mfa,
          requestId,
          abortController.signal
        );

        // Validate that it points to localhost
        validateClientRedirect(tshRedirectUrl);

        // Redirect to the validated URL
        onRedirect(tshRedirectUrl);
      } catch (err) {
        setAttempt({
          status: 'failed',
          statusText: err.message,
        });
      }
    }

    promptWebauthnAndRedirect();
    return () => abortController.abort();
  }, []); // eslint-disable-line react-hooks/exhaustive-deps -- Only run the effect once on mount

  if (attempt.status === 'failed') {
    if (attempt.statusCode === 400) {
      return <BadRequest message={attempt.statusText} />;
    }
    return <BrowserMfaAccessDenied statusText={attempt.statusText} />;
  }

  if (shouldShowMfaPrompt(mfa)) {
    return <AuthnDialog mfaState={mfa} />;
  }

  return <BrowserMfaProcessing />;
}

export function BrowserMfaProcessing() {
  return (
    <Flex height="180px" justifyContent="center" alignItems="center" flex="1">
      <Indicator />
    </Flex>
  );
}

interface BrowserMFAAccessDeniedProps {
  statusText: string;
}

export function BrowserMfaAccessDenied(props: BrowserMFAAccessDeniedProps) {
  return <AccessDenied message={props.statusText} />;
}

function redirectTo(url: string): void {
  window.location.replace(url);
}
