import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useCallback, useRef, useState } from 'react';
import { Link as RouterLink } from 'react-router';

import {
  Alert,
  Box,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  Link,
  Text,
} from 'design';
import { Check } from 'design/Icon';
import { TextSelectCopy } from 'shared/components/TextSelectCopy';
import { getErrMessage } from 'shared/utils/errorType';

import cfg from 'e-teleport/config';
import { useOktaIntegrationSetUpContext } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/SetUpContext';
import {
  ListItem,
  NumberedList,
  OktaIntegrationStepType,
  OktaSetupStepComplete,
  SCIM_CONFIG,
} from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Shared';
import type { OktaIntegrationStepFormProps } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Steps/props';
import { Header } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Shared';
import { StyledBox } from 'e-teleport/Integrations/Shared';
import { pluginsService } from 'e-teleport/services/plugins';
import { createFetchPluginQueryKey } from 'e-teleport/services/plugins/hooks';
import { Redirect } from 'teleport/components/Router';
import { withUnsupportedOktaPluginUpdateErrorConversion } from 'teleport/services/version/unsupported';

export const SetUpScim = () => {
  const {
    completedStepTypes,
    plugin,
    startFrom,
    getNextStep,
    getPreviousStep,
  } = useOktaIntegrationSetUpContext();

  if (startFrom !== undefined && startFrom !== OktaIntegrationStepType.Sso) {
    return <Redirect to={cfg.oss.getIntegrationEnrollRoute('okta')} />;
  }

  const previousStep = getPreviousStep(OktaIntegrationStepType.Scim);
  const previousStepType = completedStepTypes.includes(previousStep?.type)
    ? undefined
    : previousStep?.type;

  return (
    <ScimForm
      plugin={plugin}
      nextStepType={getNextStep(OktaIntegrationStepType.Scim).type}
      previousStepType={previousStepType}
    />
  );
};

export const ScimForm = ({
  nextStepType,
  plugin,
  previousStepType,
  isEditing,
}: OktaIntegrationStepFormProps) => {
  const scimToken = useRef(crypto.randomUUID());
  const [isComplete, setIsComplete] = useState(false);

  const queryClient = useQueryClient();

  const updatePlugin = useMutation({
    mutationFn: () =>
      pluginsService
        .updatePlugin({
          plugin: 'okta',
          okta: {
            scimToken: scimToken.current,
            enableUserSync: !!plugin.spec?.enableUserSync,
            assignDefaultRoles: !!plugin?.spec?.assignDefaultRoles,
            enableAccessListSync: !!plugin.spec?.enableAccessListSync,
            enableAppGroupSync: !!plugin.spec?.enableAppGroupSync,
            enableSystemLogExport: !!plugin.spec?.enableSystemLogExport,
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

  const onSubmit = useCallback(() => {
    if (updatePlugin.isPending) {
      return;
    }

    updatePlugin.mutate();
  }, [updatePlugin]);

  if (isComplete) {
    if (isEditing) {
      return (
        <Redirect to={cfg.oss.getIntegrationStatusRoute('okta', 'okta')} />
      );
    }

    return <OktaSetupStepComplete config={SCIM_CONFIG} />;
  }

  return (
    <Flex flexDirection="column" gap={5} maxWidth={900}>
      <Box>
        <Header header="Configure SCIM" />
        {!isEditing && (
          <Text>
            This step is optional. If you do not wish to use SCIM, click{' '}
            <b>Skip</b> at the bottom of this page.
          </Text>
        )}
      </Box>
      <Flex flexDirection="column" gap={4} width="100%">
        <StyledBox header="Configure SCIM details in the Okta app">
          <Text>
            <b>API Token</b>
            {!updatePlugin.isSuccess && !updatePlugin.isPending && ' (unsaved)'}
            :
          </Text>
          <TextSelectCopy bash={false} text={scimToken.current} />
          <NumberedList>
            <ListItem>
              <Text>
                Click <b>{isEditing ? 'Update' : 'Save'} SCIM Configuration</b>{' '}
                below to update the SCIM configuration in Teleport before
                proceeding.
                {isEditing && (
                  <>
                    {' '}
                    <b>Note</b> that this will invalidate any previously
                    generated token.
                  </>
                )}
              </Text>
              <ButtonPrimary
                onClick={onSubmit}
                disabled={updatePlugin.isSuccess || updatePlugin.isPending}
                width="fit-content"
                intent={updatePlugin.isSuccess ? 'success' : 'primary'}
                px={3}
              >
                {updatePlugin.isSuccess && <Check mr={2} size="small" />}
                {`${isEditing ? 'Update' : 'Save'} SCIM Configuration`}
              </ButtonPrimary>
            </ListItem>
            <ListItem>
              <Text>
                Using the <b>API Token</b> above, follow the instructions in the
                Teleport documentation to{' '}
                <Link
                  href="https://goteleport.com/docs/identity-governance/integrations/okta/scim-integration/#step-22-configure-scim-details-in-the-okta-app"
                  target="_blank"
                >
                  create and configure
                </Link>{' '}
                SCIM for the Teleport SAML application in the Okta dashboard.
              </Text>
            </ListItem>
          </NumberedList>
          {updatePlugin.isError && (
            <Alert kind="outline-danger" mb={0}>
              {getErrMessage(updatePlugin.error)}
            </Alert>
          )}
        </StyledBox>
      </Flex>
      <Flex flexDirection="row" alignItems="center" gap={3}>
        <ButtonPrimary
          onClick={() => setIsComplete(true)}
          disabled={!updatePlugin.isSuccess}
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
          disabled={updatePlugin.isPending}
        >
          {isEditing ? 'Cancel' : 'Back'}
        </ButtonSecondary>
        {!isEditing && !updatePlugin.isSuccess && (
          <ButtonSecondary
            as={RouterLink}
            to={cfg.oss.getIntegrationEnrollRoute('okta', nextStepType)}
            disabled={updatePlugin.isPending}
          >
            Skip
          </ButtonSecondary>
        )}
      </Flex>
    </Flex>
  );
};
