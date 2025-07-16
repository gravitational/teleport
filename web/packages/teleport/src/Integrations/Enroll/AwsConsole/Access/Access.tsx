/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { keepPreviousData, useMutation, useQuery } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { useHistory, useLocation, useParams } from 'react-router';

import * as Alerts from 'design/Alert';
import { Alert } from 'design/Alert';
import Box from 'design/Box';
import { ButtonPrimary, ButtonSecondary } from 'design/Button';
import Flex from 'design/Flex';
import { Indicator } from 'design/Indicator';
import { H2, P2 } from 'design/Text';
import { InfoGuideButton } from 'shared/components/SlidingSidePanel/InfoGuide';

import { EmptyState } from 'teleport/Bots/List/EmptyState/EmptyState';
import { FeatureBox } from 'teleport/components/Layout';
import cfg from 'teleport/config';
import { Profiles } from 'teleport/Integrations/Enroll/AwsConsole/Access/Profiles';
import { ProfilesEmptyState } from 'teleport/Integrations/Enroll/AwsConsole/Access/ProfilesEmptyState';
import { ProfilesFilterOption } from 'teleport/Integrations/Enroll/AwsConsole/Access/ProfilesFilter';
import { Guide } from 'teleport/Integrations/Enroll/AwsConsole/Guide';
import { useAwsOidcStatus } from 'teleport/Integrations/status/AwsOidc/useAwsOidcStatus';
import { ApiError } from 'teleport/services/api/parseError';
import {
  IntegrationKind,
  integrationService,
} from 'teleport/services/integrations';
import useTeleport from 'teleport/useTeleport';

export function Access() {
  const ctx = useTeleport();
  const flags = ctx.getFeatureFlags();
  const canEnroll = flags.enrollIntegrations;
  const clusterId = ctx.storeUser.getClusterId();
  const resourceRoute = cfg.getUnifiedResourcesRoute(clusterId);

  const { statsAttempt } = useAwsOidcStatus();

  const history = useHistory();
  const location = useLocation<{
    integrationName?: string;
    trustAnchorArn?: string;
    syncRoleArn?: string;
    syncProfileArn?: string;
  }>();

  //  name is the parent AWS integration name; whereas the integration name
  //  refers to the enrollment in-progress AWS CLI integration
  const { name } = useParams<{ name: string }>();
  const {
    integrationName = '',
    trustAnchorArn = '',
    syncRoleArn = '',
    syncProfileArn = '',
  } = location.state;
  const [syncAll, setSyncAll] = useState(true);
  const [filters, setFilters] = useState<ProfilesFilterOption[]>([]);

  const { status, error, data, isFetching, refetch } = useQuery({
    enabled: canEnroll,
    queryKey: ['profiles', filters],
    queryFn: () =>
      integrationService.awsRolesAnywhereProfiles({
        integrationName: name,
        filters,
      }),
    placeholderData: keepPreviousData,
    staleTime: 30_000, // Cached pages are valid for 30 seconds
  });

  const submitSync = useMutation({
    mutationFn: () =>
      integrationService
        .createIntegration({
          name: integrationName,
          subKind: IntegrationKind.AWSRa,
          awsRa: {
            trustAnchorArn: trustAnchorArn,
            profileSyncConfig: {
              enabled: true, // must be true for creation
              profileArn: syncProfileArn,
              profileAcceptsRoleSessionName: false, // not necessary for creation
              roleArn: syncRoleArn,
              profileNameFilters:
                filters.length > 0 ? filters.map(f => f.value) : ['*'],
            },
          },
        })
        .then(data => data),
    onSuccess: () => {
      history.push(
        cfg.getIntegrationEnrollChildRoute(
          IntegrationKind.AwsOidc,
          name,
          IntegrationKind.AwsConsole,
          'next'
        )
      );
    },
    onError: (e: ApiError) => {
      // Set validity on invalid filter based on API error
      const messages = e.messages.join(' ');
      const next = filters.map(f => {
        if (e.message.includes(f.value) || messages.includes(f.value)) {
          return {
            ...f,
            invalid: false,
          };
        } else {
          return f;
        }
      });

      // todo mberg setting this should re-render the filter component but does it automatically run validator?
      setFilters(next);
    },
  });

  useEffect(() => {
    // clear filters on syncAll
    if (syncAll) {
      setFilters([]);
    }
  }, [syncAll]);

  if (!canEnroll) {
    return (
      <FeatureBox>
        <Alert kind="info" mt={4}>
          You do not have permission to enroll integrations. Missing role
          permissions: <code>integrations.create</code>
        </Alert>
        <EmptyState />
      </FeatureBox>
    );
  }

  //  todo mberg test
  // verify that the parent aws oidc integration exists/is valid
  if (statsAttempt.status === 'error') {
    return (
      <Alerts.Danger details={statsAttempt.error.message}>
        Error: {statsAttempt.error.name}
      </Alerts.Danger>
    );
  }

  //  todo mberg test this failure flow
  if (!integrationName || !trustAnchorArn || !syncRoleArn || !syncProfileArn) {
    return (
      <FeatureBox>
        <Alert kind="info" mt={4}>
          Missing form data, please go back and restart.
        </Alert>
        <EmptyState />
        <ButtonSecondary
          onClick={() =>
            history.push(
              cfg.getIntegrationEnrollChildRoute(
                IntegrationKind.AwsOidc,
                name,
                IntegrationKind.AwsConsole,
                'integration'
              )
            )
          }
          width="100px"
        >
          Back
        </ButtonSecondary>
      </FeatureBox>
    );
  }

  return (
    <Box pt={3}>
      <Flex mt={3} justifyContent="space-between" alignItems="center">
        <H2>Configure Access</H2>
        <InfoGuideButton
          config={{
            guide: <Guide resourcesRoute={resourceRoute} />,
          }}
        />
      </Flex>
      <P2 mb={3}>
        Import and synchronize AWS IAM Roles Anywhere Profiles into Teleport.
        Imported Profiles will be available as Resources with each Role
        available as an account.
      </P2>
      {status === 'error' && (
        <Alerts.Danger details={error.message}>
          Error: {error.name}
        </Alerts.Danger>
      )}
      {status === 'pending' && (
        <Box data-testid="loading" textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {status === 'success' &&
        (!data.profiles || data.profiles.length === 0) && (
          <ProfilesEmptyState />
        )}
      {status === 'success' && data.profiles.length > 0 && (
        <Profiles
          data={data.profiles}
          fetchStatus={isFetching ? 'loading' : ''}
          filters={filters}
          setFilters={setFilters}
          refetch={refetch}
          syncAll={syncAll}
          setSyncAll={setSyncAll}
        />
      )}
      <Flex gap={3} mt={3}>
        {submitSync.error && (
          <Alerts.Danger details={submitSync.error?.message}>
            Error: {submitSync.error.name}
          </Alerts.Danger>
        )}
        <ButtonPrimary
          width="200px"
          disabled={!syncAll}
          onClick={() => submitSync.mutate()}
        >
          Enable Sync
        </ButtonPrimary>
        <ButtonSecondary
          onClick={() =>
            history.push(
              cfg.getIntegrationEnrollChildRoute(
                IntegrationKind.AwsOidc,
                name,
                IntegrationKind.AwsConsole,
                'integration'
              )
            )
          }
          width="100px"
        >
          Back
        </ButtonSecondary>
      </Flex>
    </Box>
  );
}
