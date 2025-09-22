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

import {
  keepPreviousData,
  useMutation,
  useQueries,
} from '@tanstack/react-query';
import { useEffect, useRef, useState } from 'react';
import { useHistory, useLocation } from 'react-router';

import { Box, CardTile, Indicator, ResourceIcon } from 'design';
import * as Alerts from 'design/Alert';
import { Alert } from 'design/Alert';
import { ButtonBorder, ButtonPrimary, ButtonSecondary } from 'design/Button';
import Flex from 'design/Flex';
import * as Icons from 'design/Icon';
import { H2, H3, P2 } from 'design/Text';
import { InfoGuideButton } from 'shared/components/SlidingSidePanel/InfoGuide';

import { FeatureBox } from 'teleport/components/Layout';
import cfg from 'teleport/config';
import { Profiles } from 'teleport/Integrations/Enroll/AwsConsole/Access/Profiles';
import {
  makeProfilesFilterOption,
  ProfilesFilterOption,
} from 'teleport/Integrations/Enroll/AwsConsole/Access/ProfilesFilter';
import { Guide } from 'teleport/Integrations/Enroll/AwsConsole/Guide';
import { rolesAnywhereCreateProfile } from 'teleport/Integrations/Enroll/awsLinks';
import { ApiError } from 'teleport/services/api/parseError';
import {
  IntegrationAwsRa,
  IntegrationKind,
  integrationService,
} from 'teleport/services/integrations';
import useTeleport from 'teleport/useTeleport';

export function Access() {
  const ctx = useTeleport();
  const integrationsAccess = ctx.storeUser.getIntegrationsAccess();
  const canEnroll = integrationsAccess.create;

  const history = useHistory();
  const location = useLocation<{
    integrationName?: string;
    trustAnchorArn?: string;
    syncProfileArn?: string;
    syncRoleArn?: string;
    edit?: boolean;
  }>();

  const {
    integrationName = '',
    trustAnchorArn = '',
    syncProfileArn = '',
    syncRoleArn = '',
    edit = false,
  } = location.state;
  const [syncAll, setSyncAll] = useState(!edit);
  const [filters, setFilters] = useState<ProfilesFilterOption[]>([]);
  // initialFilters is used in the edit flow to reset filters if import all is un-toggled
  const [initialFilters, setInitialFilters] = useState<ProfilesFilterOption[]>(
    []
  );
  // initialComplete & foundProfiles allows us to show a custom view for zero profiles,
  // vs. showing the table view when the user has filtered down to zero
  const initialComplete = useRef(false);
  const [foundProfiles, setFoundProfiles] = useState(false);

  const [intervalMs, setIntervalMs] = useState<number | false>(
    edit ? false : 2000
  );
  const results = useQueries({
    queries: [
      // The integration endpoint is used for editing an existing subscription. It is only enabled if editing and here
      // is no refetch logic, the response is used to set existing filters if they exist on the integration.
      {
        queryKey: ['integration'],
        enabled: edit,
        gcTime: 0,
        placeholderData: keepPreviousData,
        refetchInterval: false,
        queryFn: () =>
          integrationService
            .fetchIntegration<IntegrationAwsRa>(integrationName)
            .then(data => {
              setFilters(
                makeProfilesFilterOption(data.spec.profileSyncConfig.filters)
              );
              setInitialFilters(
                makeProfilesFilterOption(data.spec.profileSyncConfig.filters)
              );
              return data;
            }),
      },
      // The profiles endpoint is called immediately after creation. Because the integration may not yet be ready, we
      // add refetchInterval which is set via intervalMs when we receive a 404 response. If a success or non-404
      // response is received, retries will end.
      {
        queryKey: ['profiles', filters],
        gcTime: 0,
        placeholderData: keepPreviousData,
        refetchInterval: intervalMs,
        queryFn: () =>
          integrationService
            .awsRolesAnywhereProfiles({
              integrationName: integrationName,
              filters,
            })
            .then(data => {
              setIntervalMs(false);
              if (!initialComplete.current) {
                setFoundProfiles(data.profiles && data?.profiles.length > 0);
                initialComplete.current = true;
              }

              return data;
            }),
      },
    ],
  });

  const [integrationResp, profilesResp] = results;
  const { status: editStatus, error: editError } = integrationResp;
  const { status, error, data, isFetching, refetch, isRefetching } =
    profilesResp;

  // Set retry logic for profiles not found errors
  const isNotFoundErr =
    error instanceof ApiError && error.response.status === 404;
  if (error && !isNotFoundErr && intervalMs != false) {
    // if the error returned is not a 404, stop the retry
    setIntervalMs(false);
  }

  const update = useMutation({
    mutationFn: () =>
      integrationService
        .updateIntegration(integrationName, {
          kind: IntegrationKind.AwsRa,
          awsRa: {
            trustAnchorARN: trustAnchorArn,
            profileSyncConfig: {
              enabled: true, // must be true for enable step; otherwise profiles won't sync
              profileArn: syncProfileArn,
              roleArn: syncRoleArn,
              filters: filters.length > 0 ? filters.map(f => f.value) : ['*'],
            },
          },
        })
        .then(data => data),
    onSuccess: () => {
      history.push(
        cfg.getIntegrationEnrollRoute(IntegrationKind.AwsRa, 'next'),
        { integrationName: integrationName }
      );
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

    // if editing, when clearing sync all reset to the initial filters
    if (!syncAll && edit) {
      setFilters(initialFilters);
    }
  }, [syncAll]);

  if (!canEnroll) {
    return (
      <FeatureBox>
        <Alert
          kind="info"
          mt={4}
          secondaryAction={{
            content: 'Back',
            onClick: history.goBack,
          }}
        >
          You do not have permission to enroll integrations. Missing role
          permissions: <code>integrations.create</code>
        </Alert>
      </FeatureBox>
    );
  }

  if (
    integrationName === '' ||
    trustAnchorArn === '' ||
    syncProfileArn === '' ||
    syncRoleArn === ''
  ) {
    return (
      <FeatureBox>
        <Alert
          kind="info"
          mt={4}
          secondaryAction={{
            content: 'Back',
            onClick: history.goBack,
          }}
        >
          Missing form data, please try again.
        </Alert>
      </FeatureBox>
    );
  }

  if (editStatus === 'error' && editError) {
    return (
      <Alerts.Danger
        details={editError.message}
        secondaryAction={{
          content: 'Back',
          onClick: history.goBack,
        }}
      >
        Unable to edit integration: {editError.name}
      </Alerts.Danger>
    );
  }

  return (
    <>
      <Flex mt={3} justifyContent="space-between" alignItems="center">
        <H2>Configure Access</H2>
        <InfoGuideButton
          config={{
            guide: <Guide />,
          }}
        />
      </Flex>
      <Box>
        <P2 mb={3}>
          Import and synchronize AWS IAM Roles Anywhere Profiles into Teleport.
          Imported Profiles will be available as Resources with each Role
          available as an account.
        </P2>
      </Box>
      {!edit && status !== 'success' && intervalMs && (
        <Alerts.Info details="It may take a moment for your integration to be ready before configuring access. This page will reload when ready.">
          We&#39;re creating your integration
        </Alerts.Info>
      )}
      {status === 'error' && !isNotFoundErr && (
        <Alerts.Danger details={error.message}>
          Error: {error.name}
        </Alerts.Danger>
      )}
      {/* only show the indicator for the first fetch */}
      {status === 'pending' && !initialComplete && (
        <Box data-testid="loading" textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {status === 'success' && !foundProfiles && <ProfilesEmptyState />}
      {foundProfiles && (
        <Profiles
          data={data?.profiles}
          fetchStatus={isFetching || isRefetching ? 'loading' : ''}
          filters={filters}
          setFilters={setFilters}
          refetch={refetch}
          syncAll={syncAll}
          setSyncAll={setSyncAll}
          isRefetching={isRefetching}
        />
      )}
      {update.error && (
        <Alerts.Danger details={update.error?.message} mt={2}>
          Error: {update.error.name}
        </Alerts.Danger>
      )}
      <Flex gap={3} mt={3}>
        <ButtonPrimary
          width="200px"
          onClick={() => update.mutate()}
          disabled={status !== 'success'}
        >
          {edit ? 'Update Sync' : 'Enable Sync'}
        </ButtonPrimary>
        {edit && (
          <ButtonSecondary onClick={history.goBack} width="100px">
            Cancel
          </ButtonSecondary>
        )}
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
      </Flex>
      <Flex gap={3}>
        <ButtonPrimary
          gap={2}
          as="a"
          target="blank"
          href={rolesAnywhereCreateProfile}
        >
          Create AWS Profiles
          <Icons.NewTab size="small" />
        </ButtonPrimary>
        <ButtonBorder intent="primary">Refresh AWS Profiles</ButtonBorder>
      </Flex>
    </CardTile>
  );
}
