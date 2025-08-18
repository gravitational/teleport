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
import { useHistory, useLocation } from 'react-router';

import { CardTile, ResourceIcon } from 'design';
import * as Alerts from 'design/Alert';
import { Alert } from 'design/Alert';
import Box from 'design/Box';
import { ButtonBorder, ButtonPrimary, ButtonSecondary } from 'design/Button';
import Flex from 'design/Flex';
import { Indicator } from 'design/Indicator';
import { H2, H3, P2 } from 'design/Text';
import { InfoGuideButton } from 'shared/components/SlidingSidePanel/InfoGuide';

import { FeatureBox } from 'teleport/components/Layout';
import cfg from 'teleport/config';
import { Guide } from 'teleport/Integrations/Enroll/AwsConsole/Access/Guide';
import { Profiles } from 'teleport/Integrations/Enroll/AwsConsole/Access/Profiles';
import { ProfilesFilterOption } from 'teleport/Integrations/Enroll/AwsConsole/Access/ProfilesFilter';
import { rolesAnywhereCreateProfile } from 'teleport/Integrations/Enroll/awsLinks';
import { ApiError } from 'teleport/services/api/parseError';
import {
  IntegrationKind,
  integrationService,
} from 'teleport/services/integrations';
import useTeleport from 'teleport/useTeleport';

export function Access() {
  const ctx = useTeleport();
  const integrationsAccess = ctx.storeUser.getIntegrationsAccess();
  const canEnroll = integrationsAccess.create;
  const clusterId = ctx.storeUser.getClusterId();
  const resourceRoute = cfg.getUnifiedResourcesRoute(clusterId);

  const history = useHistory();
  const location = useLocation<{
    integrationName?: string;
    trustAnchorArn?: string;
    syncRoleArn?: string;
    syncProfileArn?: string;
  }>();

  const {
    integrationName = '',
    trustAnchorArn = '',
    syncRoleArn = '',
    syncProfileArn = '',
  } = location.state;
  const [syncAll, setSyncAll] = useState(true);
  const [filters, setFilters] = useState<ProfilesFilterOption[]>([]);

  // todo (michellescripts) list profiles as written today fails
  //   meeting with marcoandredinis next week to correct
  const { status, error, data, isFetching, refetch } = useQuery({
    enabled: canEnroll,
    queryKey: ['profiles', filters],
    gcTime: 0,
    queryFn: () =>
      integrationService.awsRolesAnywhereProfiles({
        integrationName: integrationName,
        filters,
      }),
    placeholderData: keepPreviousData,
    staleTime: 30_000, // Cached pages are valid for 30 seconds
  });

  // todo (michellescripts) create as written today fails
  //   meeting with marcoandredinis next week to correct
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
      //   todo (michellescripts) redirect to success view in follow up PR
    },
    onError: (e: ApiError) => {
      // Set validity on invalid filter based on API error
      const messages = e.messages.join(' ');
      const next = filters.map(f => {
        if (e.message.includes(f.value) || messages.includes(f.value)) {
          return {
            ...f,
            invalid: true,
          };
        } else {
          return f;
        }
      });

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
      </FeatureBox>
    );
  }

  if (!integrationName || !trustAnchorArn || !syncRoleArn || !syncProfileArn) {
    return (
      <FeatureBox>
        <Alert kind="info" mt={4}>
          Missing form data, please go back and restart enrollment.
        </Alert>
        <ButtonPrimary
          onClick={() =>
            history.push(
              cfg.getIntegrationEnrollRoute(
                IntegrationKind.AWSRa,
                'integration'
              )
            )
          }
          width="100px"
        >
          Back
        </ButtonPrimary>
      </FeatureBox>
    );
  }

  const nothingToSync =
    status === 'success' && (!data.profiles || data.profiles.length === 0);
  return (
    <>
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
      {nothingToSync && <ProfilesEmptyState />}
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
      {submitSync.error && (
        <Alerts.Danger details={submitSync.error?.message} mt={2}>
          Error: {submitSync.error.name}
        </Alerts.Danger>
      )}
      <Flex gap={3} mt={3}>
        <ButtonPrimary
          width="200px"
          onClick={() => submitSync.mutate()}
          disabled={nothingToSync}
        >
          Enable Sync
        </ButtonPrimary>
        <ButtonSecondary
          onClick={() =>
            history.push(
              cfg.getIntegrationEnrollRoute(
                IntegrationKind.AWSRa,
                'integration'
              )
            )
          }
          width="100px"
        >
          Back
        </ButtonSecondary>
      </Flex>
    </>
  );
}

function ProfilesEmptyState() {
  return (
    <CardTile alignItems="center" gap={4}>
      <ResourceIcon name="awsidentityandaccessmanagementiam" width="164px" />
      <Flex flexDirection="column" alignItems="center">
        <H3 mb={1}>No AWS IAM Roles Anywhere Profiles Found</H3>
        <P2>Create AWS IAM Roles Anywhere Profiles in your AWS console</P2>
      </Flex>
      <Flex gap={3}>
        <ButtonPrimary as="a" target="blank" href={rolesAnywhereCreateProfile}>
          Create AWS Roles Anywhere Profiles
        </ButtonPrimary>
        <ButtonBorder intent="primary">
          Refresh AWS Roles Anywhere Profiles
        </ButtonBorder>
      </Flex>
    </CardTile>
  );
}
