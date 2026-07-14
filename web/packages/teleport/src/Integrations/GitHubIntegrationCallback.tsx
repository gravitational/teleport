/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { useEffect, useState } from 'react';
import { useLocation } from 'react-router';

import { Alert, Card, H1, Indicator, Box, P1 } from 'design';

import { LogoHero } from 'teleport/components/LogoHero';
import cfg from 'teleport/config';
import api from 'teleport/services/api';

/**
 * GitHubIntegrationCallback handles the authenticated GitHub OAuth callback for
 * git integration. GitHub redirects here after the user authorizes. This page
 * requires a valid web session, then POSTs code/state to the API endpoint which
 * verifies the session user matches the auth request user.
 */
export function GitHubIntegrationCallback() {
  const location = useLocation();
  const [error, setError] = useState('');
  const [processing, setProcessing] = useState(true);

  useEffect(() => {
    const params = new URLSearchParams(location.search);
    const code = params.get('code');
    const state = params.get('state');

    if (!code || !state) {
      setError('Missing required parameters from GitHub.');
      setProcessing(false);
      return;
    }

    api
      .post(cfg.api.githubIntegrationCallbackPath, { code, state })
      .then((resp: { redirectURL?: string }) => {
        if (resp.redirectURL) {
          window.location.replace(resp.redirectURL);
          return;
        }
        setProcessing(false);
      })
      .catch(err => {
        setError(err.message || 'An unknown error occurred.');
        setProcessing(false);
      });
  }, [location.search]);

  return (
    <GitHubIntegrationCallbackView processing={processing} error={error} />
  );
}

export function GitHubIntegrationCallbackView({
  processing,
  error,
}: {
  processing: boolean;
  error: string;
}) {
  return (
    <>
      <LogoHero />
      <Card
        color="text.main"
        bg="levels.elevated"
        width="540px"
        mx="auto"
        my={6}
        p={5}
      >
        {processing && (
          <>
            <H1 mb={4} textAlign="center">
              Authorizing with GitHub
            </H1>
            <Box textAlign="center">
              <Indicator size="large" />
              <P1 mt={3}>Please wait...</P1>
            </Box>
          </>
        )}
        {!processing && error && (
          <>
            <H1 mb={4} textAlign="center">
              GitHub Authorization Failed
            </H1>
            <Alert mt={2} mb={4}>
              {error}
            </Alert>
          </>
        )}
        {!processing && !error && (
          <>
            <H1 mb={4} textAlign="center">
              Authorization Complete
            </H1>
            <P1 textAlign="center">
              Your GitHub account has been linked. You may close this tab.
            </P1>
          </>
        )}
      </Card>
    </>
  );
}
