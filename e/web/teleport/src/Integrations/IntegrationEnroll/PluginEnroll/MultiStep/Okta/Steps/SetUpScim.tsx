import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useCallback, useRef, useState } from 'react';
import { Link } from 'react-router-dom';

import { Alert, Box, ButtonPrimary, ButtonSecondary, Flex, Text } from 'design';
import { Check } from 'design/Icon';
import { TextSelectCopy } from 'shared/components/TextSelectCopy';
import { getErrMessage } from 'shared/utils/errorType';

import cfg from 'e-teleport/config';
import { useOktaIntegrationSetUpContext } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/SetUpContext';
import {
  BulletList,
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

// TODO(lisa): this is hard coded for now and is equal to backend
// hard coded value. Next iteration we will allow user to name
// the SSO connector.
const SSO_CONNECTOR_NAME = 'okta';

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
        <Header header="Configure SCIM in Okta’s Admin Console" />
        <Text>
          You will need to take all of the following steps in the Okta admin
          console.
        </Text>
      </Box>
      <Flex flexDirection="column" gap={4} width="100%">
        <StyledBox header="Step 1: Enable SCIM in Okta">
          <NumberedList>
            <ListItem>
              <Text>
                In the Okta dashboard, go to the navigation bar and click{' '}
                <b>Applications</b>, then <b>Applications</b>, and then click on
                your Teleport SAML application.
              </Text>
            </ListItem>
            <ListItem>
              <Text>
                Click on the <b>General</b> tab. Under <b>App Settings</b>,
                click <b>Edit</b>, and check the <b>SCIM</b> box under{' '}
                <b>Provisioning</b>.
              </Text>
            </ListItem>
            <ListItem>
              <Text>
                Click <b>Save</b> to update the app.
              </Text>
            </ListItem>
          </NumberedList>
        </StyledBox>
        <StyledBox header="Step 2: Configure SCIM Details in Okta">
          <NumberedList>
            <ListItem>
              <Text>
                Open the <b>Provisioning</b> tab and click <b>Edit</b>.
              </Text>
            </ListItem>
            <ListItem>
              <Text>
                Copy the URL below and paste it into the{' '}
                <b>SCIM connector base URL</b> field:
              </Text>
              <TextSelectCopy
                bash={false}
                text={`${cfg.oss.baseUrl}/v1/webapi/scim/${SSO_CONNECTOR_NAME}`}
              />
            </ListItem>
            <ListItem>
              <Text>
                Copy the value below and paste it into the{' '}
                <b>Unique identifier field for users</b> in Okta:
              </Text>
              <TextSelectCopy bash={false} text="userName" />
            </ListItem>
            <ListItem>
              <Text>
                In Okta, check the following boxes under{' '}
                <b>Supported provisioning actions</b>:
              </Text>
              <BulletList>
                <li>
                  <b>Import New Users and Profile Updates</b>
                </li>
                <li>
                  <b>Push New Users</b>
                </li>
                <li>
                  <b>Push Profile Updates</b>
                </li>
              </BulletList>
            </ListItem>
            <ListItem>
              <Text>
                Open the <b>Authentication Mode</b> drop-down menu and select{' '}
                <b>HTTP Header</b>.
              </Text>
            </ListItem>
            <ListItem>
              <Text>
                Copy the value below and paste it into the <b>Bearer Token</b>{' '}
                field:
              </Text>
              <TextSelectCopy bash={false} text={scimToken.current} />
            </ListItem>
            <ListItem>
              <Text>
                Click <b>{isEditing ? 'Update' : 'Save'} SCIM Configuration</b>{' '}
                below to update the SCIM configuration in Teleport before
                proceeding.
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
                In Okta, click <b>Test Connector Configuration</b> to confirm
                all of the details are set up correctly, and then click{' '}
                <b>Save</b>.
              </Text>
            </ListItem>
          </NumberedList>
          {updatePlugin.isError && (
            <Alert kind="outline-danger" mb={0}>
              {getErrMessage(updatePlugin.error)}
            </Alert>
          )}
        </StyledBox>
        <StyledBox header="Step 3: Configure SCIM Provisioning Permissions in Okta">
          <NumberedList>
            <ListItem>
              <Text>
                Stay on the <b>Provisioning</b> tab, and you should see that the
                previous step added a <b>To App</b> tab in the side menu. Open
                it.
              </Text>
            </ListItem>
            <ListItem>
              <Text>Enable the following checkboxes:</Text>
              <BulletList>
                <li>
                  <b>Create Users</b>
                </li>
                <li>
                  <b>Update User Attributes</b>
                </li>
                <li>
                  <b>Deactivate Users</b>
                </li>
              </BulletList>
            </ListItem>
            <ListItem>
              <Text>
                Click <b>Save</b>.
              </Text>
            </ListItem>
          </NumberedList>
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
          as={Link}
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
            as={Link}
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
