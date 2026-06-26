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
import { useParams } from 'react-router-dom';

import { Box, Indicator, Text } from 'design';
import { Danger } from 'design/Alert';
import CardIcon from 'design/CardIcon';

import { LogoHero } from 'teleport/components/LogoHero';
import cfg from 'teleport/config';
import api from 'teleport/services/api';

/**
 * GitHubIntegrationLogin initiates the GitHub OAuth flow for the given
 * organization. Used as a standalone page for headless environments (e.g.
 * Beams) where the user opens this URL in a browser to authorize GitHub
 * access.
 */
export function GitHubIntegrationLogin() {
  const { org } = useParams<{ org: string }>();
  const [error, setError] = useState('');

  useEffect(() => {
    if (!org) {
      setError('Missing organization parameter.');
      return;
    }

    api
      .post(cfg.api.githubIntegrationLoginPath, { organization: org })
      .then((resp: { redirectUrl?: string }) => {
        if (resp.redirectUrl) {
          window.location.href = resp.redirectUrl;
        } else {
          setError('No redirect URL received.');
        }
      })
      .catch(err => {
        setError(err.message || 'Failed to start GitHub authorization.');
      });
  }, [org]);

  return (
    <>
      <LogoHero />
      {!error && (
        <CardIcon title="GitHub Authorization">
          <Box textAlign="center">
            <Indicator size="large" />
            <Text mt={3}>Redirecting to GitHub...</Text>
          </Box>
        </CardIcon>
      )}
      {error && (
        <CardIcon title="GitHub Authorization Failed">
          <Danger>{error}</Danger>
        </CardIcon>
      )}
    </>
  );
}
