import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useCallback, useState } from 'react';
import { Link as RouterLink } from 'react-router-dom';

import { Alert, Box, ButtonPrimary, ButtonSecondary, Flex, Text } from 'design';
import { getErrMessage } from 'shared/utils/errorType';

import cfg from 'e-teleport/config';
import { useOktaIntegrationSetUpContext } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/SetUpContext';
import {
  BulletList,
  IDENTITY_SECURITY_SYNC_CONFIG,
  ListItem,
  OktaIntegrationStepType,
  OktaSetupStepComplete,
} from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Shared';
import type { OktaIntegrationStepFormProps } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Steps/props';
import { RequiredPermissionItem } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Steps/SetUpUserSync';
import { Header } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Shared';
import { StyledBox } from 'e-teleport/Integrations/Shared';
import { pluginsService } from 'e-teleport/services/plugins';
import { createFetchPluginQueryKey } from 'e-teleport/services/plugins/hooks';
import { Redirect } from 'teleport/components/Router';
import { withUnsupportedOktaPluginUpdateErrorConversion } from 'teleport/services/version/unsupported';

export function SetupIdentitySecuritySync() {
  const { plugin, startFrom, getNextStep, getPreviousStep } =
    useOktaIntegrationSetUpContext();

  if (startFrom === OktaIntegrationStepType.IdentitySecuritySync) {
    return <Redirect to={cfg.oss.getIntegrationEnrollRoute('okta')} />;
  }

  return (
    <SetupIdentitySecuritySyncForm
      plugin={plugin}
      nextStepType={
        getNextStep(OktaIntegrationStepType.IdentitySecuritySync).type
      }
      previousStepType={
        getPreviousStep(OktaIntegrationStepType.IdentitySecuritySync).type
      }
    />
  );
}

export function SetupIdentitySecuritySyncForm({
  isEditing,
  nextStepType,
  plugin,
  previousStepType,
}: OktaIntegrationStepFormProps) {
  const [isComplete, setIsComplete] = useState(false);

  const queryClient = useQueryClient();

  const isEnabled = plugin.spec?.enableSystemLogExport;

  const toggleSystemLogExport = useMutation({
    mutationFn: () =>
      pluginsService
        .updatePlugin({
          plugin: 'okta',
          okta: {
            enableUserSync: !!plugin.spec?.enableUserSync,
            assignDefaultRoles: !!plugin?.spec?.assignDefaultRoles,
            enableAccessListSync: !!plugin.spec?.enableAccessListSync,
            enableAppGroupSync: !!plugin.spec?.enableAppGroupSync,
            enableSystemLogExport: !isEnabled,
            enableBidirectionalSync: !!plugin.spec?.enableBidirectionalSync,
            defaultOwners: plugin.spec?.defaultOwners,
            appFilters:
              plugin.status?.details?.accessListsSyncDetails?.appFilters,
            groupFilters:
              plugin.status?.details?.accessListsSyncDetails?.groupFilters,
          },
        })
        .catch(withUnsupportedOktaPluginUpdateErrorConversion),
    onSuccess: data =>
      queryClient.setQueryData(createFetchPluginQueryKey('okta'), data),
  });

  const onToggleSync = useCallback(() => {
    if (toggleSystemLogExport.isPending) {
      return;
    }

    toggleSystemLogExport.mutate();
  }, [toggleSystemLogExport]);

  const onSubmit = useCallback(() => {
    if (toggleSystemLogExport.isPending) {
      return;
    }

    setIsComplete(true);
  }, [toggleSystemLogExport.isPending]);

  if (isComplete) {
    if (isEditing) {
      return (
        <Redirect to={cfg.oss.getIntegrationStatusRoute('okta', 'okta')} />
      );
    }

    return <OktaSetupStepComplete config={IDENTITY_SECURITY_SYNC_CONFIG} />;
  }

  return (
    <Flex flexDirection="column" gap={5} maxWidth={900}>
      <Box>
        <Header header="Sync Okta Audit Logs with Teleport Identity Security" />
        <Text>
          Automatically ingest Okta’s Audit Logs to gain complete visibility
          into identity activity. Enhance threat detection, streamline
          investigations, and correlate events across your infrastructure.
        </Text>
      </Box>
      <Flex flexDirection="column" gap={4} width="100%">
        <StyledBox header="Enable Sync">
          <BulletList>
            {isEnabled ? (
              <ListItem>
                <Text>
                  Okta Audit Logs are now synced with Teleport Identity Security
                </Text>
              </ListItem>
            ) : (
              <>
                <ListItem>
                  <Text>
                    To enable Okta Audit Log syncing with Teleport Identity
                    Security, add the following scopes to the Okta application
                    used for user synchronization.
                  </Text>

                  <BulletList>
                    {identitySecuritySyncRequiredScopes.map(([scope, desc]) => (
                      <RequiredPermissionItem
                        key={scope}
                        text={scope}
                        description={desc}
                      />
                    ))}
                  </BulletList>
                </ListItem>

                <ListItem>
                  <Text fontWeight="bold">
                    Optional: Import users with elevated permissions and their
                    tokens
                  </Text>
                  <Text>
                    Edit the role assignment for the Okta application. Click{' '}
                    <strong>Add assignment</strong> and add the{' '}
                    <strong>Super Administrator</strong> role. Click{' '}
                    <strong>Save Changes</strong>.
                  </Text>
                </ListItem>
              </>
            )}
          </BulletList>

          <ButtonPrimary
            onClick={onToggleSync}
            disabled={toggleSystemLogExport.isPending}
            width="fit-content"
            intent={isEnabled ? 'danger' : 'success'}
            px={3}
          >
            {isEnabled ? 'Disable' : 'Enable'} Sync
          </ButtonPrimary>

          {toggleSystemLogExport.isError && (
            <Box maxWidth={800}>
              <Alert kind="danger" mb={0}>
                {getErrMessage(toggleSystemLogExport.error)}
              </Alert>
            </Box>
          )}
        </StyledBox>
      </Flex>
      <Flex flexDirection="row" alignItems="center" gap={3}>
        <ButtonPrimary
          onClick={onSubmit}
          disabled={
            (!isEditing && !isEnabled) || toggleSystemLogExport.isPending
          }
        >
          {isEditing ? 'Done' : 'Continue'}
        </ButtonPrimary>
        <ButtonSecondary
          as={RouterLink}
          to={
            isEditing
              ? cfg.oss.getIntegrationStatusRoute('okta', 'okta')
              : cfg.oss.getIntegrationEnrollRoute('okta', previousStepType)
          }
          disabled={toggleSystemLogExport.isPending}
        >
          {isEditing ? 'Cancel' : 'Back'}
        </ButtonSecondary>
        {!isEditing && !isEnabled && (
          <ButtonSecondary
            as={RouterLink}
            to={cfg.oss.getIntegrationEnrollRoute('okta', nextStepType)}
            disabled={toggleSystemLogExport.isPending}
          >
            Skip
          </ButtonSecondary>
        )}
      </Flex>
    </Flex>
  );
}

type RequiredScope = [string, string];

const identitySecuritySyncRequiredScopes: RequiredScope[] = [
  [
    'okta.apiTokens.read',
    'Required to read information about API tokens in your Okta organization',
  ],
  ['okta.logs.read', 'Required to read your Okta organization logs'],
  [
    'okta.orgs.read',
    'Required to read information about your Okta organization',
  ],
  [
    'okta.roles.read',
    'Required to read information about roles in your Okta organization',
  ],
] as const;
