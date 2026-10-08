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

import { useMutation, useQuery } from '@tanstack/react-query';
import { useParams } from 'react-router';

import { Alert, Box, ButtonPrimary, Card, H1, Indicator, P1 } from 'design';
import { getErrorMessage } from 'shared/utils/error';

import { LogoHero } from 'teleport/components/LogoHero';
import cfg from 'teleport/config';
import api from 'teleport/services/api';
import makeGitServer from 'teleport/services/gitServers/makeGitServer';

/**
 * GitServerAuthorize asks the user to confirm before starting the GitHub OAuth
 * flow for a git server. The flow is not started automatically so a crafted
 * link cannot complete an authorization the user did not initiate.
 */
export function GitServerAuthorize({
  onRedirect = (url: string) => window.location.replace(url),
}: {
  // onRedirect is used for testing only.
  onRedirect?: (url: string) => void;
}) {
  const { name } = useParams<{ name: string }>();

  const gitServer = useQuery({
    queryKey: ['gitServer', name],
    queryFn: ({ signal }) =>
      api
        .get(
          cfg.getGitServerUrl({ clusterId: cfg.proxyCluster, name }, 'get'),
          signal
        )
        .then(makeGitServer),
  });

  const { mutate, isPending, isSuccess, error } = useMutation({
    mutationFn: () =>
      api.post(
        cfg.getGitServerUrl({ clusterId: cfg.proxyCluster, name }, 'authorize')
      ),
    onSuccess: (resp: { redirectURL: string }) => {
      onRedirect(resp.redirectURL);
    },
  });

  return (
    <>
      <LogoHero />
      <GitServerAuthorizeView
        organization={gitServer.data?.github?.organization}
        processing={isPending || isSuccess}
        error={
          (gitServer.error && getErrorMessage(gitServer.error)) ||
          (error && getErrorMessage(error))
        }
        onAuthorize={() => mutate()}
      />
    </>
  );
}

export function GitServerAuthorizeView({
  organization,
  processing,
  error,
  onAuthorize,
}: {
  organization?: string;
  processing: boolean;
  error: string;
  onAuthorize(): void;
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
        Authorize with GitHub
      </H1>
      {error && (
        <Alert mt={2} mb={4}>
          {error}
        </Alert>
      )}
      <P1 mb={4} textAlign="center">
        Continue to GitHub to authorize Teleport to access organization{' '}
        <strong>{organization}</strong>.
      </P1>
      <ButtonPrimary
        block
        size="large"
        disabled={!organization || processing}
        onClick={onAuthorize}
      >
        {processing ? (
          <Box textAlign="center">
            <Indicator size="large" color="" />
          </Box>
        ) : (
          'Continue'
        )}
      </ButtonPrimary>
    </Card>
  );
}
