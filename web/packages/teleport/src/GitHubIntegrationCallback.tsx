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

import { Indicator, Box, Text } from 'design';
import { Danger } from 'design/Alert';
import CardIcon from 'design/CardIcon';
import { CircleCheck } from 'design/Icon';

import { LogoHero } from 'teleport/components/LogoHero';
import cfg from 'teleport/config';
import api from 'teleport/services/api';

/**
 * GitHubIntegrationCallback handles the authenticated GitHub OAuth callback for
 * git integration. The old callback at /v1/webapi/github/callback redirects
 * here when the auth request has AuthenticatedUser set. This page is behind
 * auth (requires web session), then POSTs code/state to the API endpoint which
 * verifies the session user matches the auth request user.
 *
 * On success, redirects to tsh's local callback server to complete the flow.
 */
export function GitHubIntegrationCallback() {
  const [error, setError] = useState('');
  const [processing, setProcessing] = useState(true);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const code = params.get('code');
    const state = params.get('state');

    if (!code || !state) {
      setError('Missing required parameters from GitHub.');
      setProcessing(false);
      return;
    }

    api
      .post(cfg.api.githubIntegrationCallbackPath, { code, state })
      .then((resp: { redirect_url?: string }) => {
        // Notify the opener tab (connect dialog) that OAuth completed.
        try {
          const channel = new BroadcastChannel('github-oauth-complete');
          channel.postMessage('done');
          channel.close();
        } catch {
          // BroadcastChannel not supported, dialog will use manual refresh.
        }

        if (resp.redirect_url) {
          window.location.replace(resp.redirect_url);
        } else {
          setProcessing(false);
        }
      })
      .catch(err => {
        setError(err.message || 'An unknown error occurred.');
        setProcessing(false);
      });
  }, []);

  return (
    <>
      <LogoHero />
      {processing && (
        <CardIcon title="GitHub Integration">
          <Box textAlign="center">
            <Indicator size="large" />
            <Text mt={3}>Completing GitHub integration...</Text>
          </Box>
        </CardIcon>
      )}
      {!processing && error && (
        <CardIcon title="GitHub Integration Failed">
          <Danger>{error}</Danger>
        </CardIcon>
      )}
      {!processing && !error && (
        <CardIcon
          title="GitHub Integration Complete"
          icon={<CircleCheck mb={3} size={64} color="success.main" />}
        >
          Successfully linked your GitHub account. You can close this window.
        </CardIcon>
      )}
    </>
  );
}
