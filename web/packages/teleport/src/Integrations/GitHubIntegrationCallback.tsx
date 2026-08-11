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

import { useMutation } from '@tanstack/react-query';
import { ReactNode, useEffect } from 'react';
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
  const params = new URLSearchParams(location.search);

  const { mutate, isPending, error } = useMutation({
    mutationFn: () =>
      api.post(cfg.api.githubIntegrationCallbackPath, {
        code: params.get('code'),
        state: params.get('state'),
      }),
    onSuccess: (resp: { redirectURL?: string }) => {
      if (resp.redirectURL) {
        window.location.replace(resp.redirectURL);
      }
    },
  });

  useEffect(() => {
    mutate();
  }, [mutate]);

  return (
    <>
      <LogoHero />
      <GitHubIntegrationCallbackView
        processing={isPending}
        error={error instanceof Error ? error.message : ''}
      />
    </>
  );
}

export function GitHubIntegrationCallbackView({
  processing,
  error,
}: {
  processing: boolean;
  error: string;
}) {
  if (processing) {
    return (
      <GitHubIntegrationCallbackCard title="Authorizing with GitHub">
        <Box textAlign="center">
          <Indicator size="large" delay="none" />
          <P1 mt={3}>Please wait...</P1>
        </Box>
      </GitHubIntegrationCallbackCard>
    );
  }

  if (error) {
    return (
      <GitHubIntegrationCallbackCard title="GitHub Authorization Failed">
        <Alert mt={2} mb={4}>
          {error}
        </Alert>
      </GitHubIntegrationCallbackCard>
    );
  }

  return (
    <GitHubIntegrationCallbackCard title="Authorization Complete">
      <P1 textAlign="center">
        Your GitHub account has been linked. You may close this tab.
      </P1>
    </GitHubIntegrationCallbackCard>
  );
}

function GitHubIntegrationCallbackCard(props: {
  title: string;
  children: ReactNode;
}) {
  return (
    <Card
      color="text.main"
      bg="levels.elevated"
      width="540px"
      mx="auto"
      my={6}
      p={5}
    >
      <H1 mb={4} textAlign="center">
        {props.title}
      </H1>
      {props.children}
    </Card>
  );
}
