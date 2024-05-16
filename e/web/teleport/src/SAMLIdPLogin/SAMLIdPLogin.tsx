/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { Flex, Indicator } from 'design';

import { AccessDenied } from 'design/CardError';

import useAttempt from 'shared/hooks/useAttemptNext';
import { isAbortError } from 'shared/utils/abortError';
import history from 'teleport/services/history';
import { parsePath } from 'history';

import auth, { MfaChallengeScope } from 'teleport/services/auth/auth';

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
        const mfaResponseJSON = JSON.stringify({
          webauthnAssertionResponse: webauthnResponse,
        });

        // Add the mfa response as a query param while preserving
        // existing query params (saml request).
        let { pathname, search } = parsePath(history.getRedirectParam());
        if (search) {
          search = `${search}&webauthn=${mfaResponseJSON}`;
        } else {
          search = `?webauthn=${mfaResponseJSON}`;
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
