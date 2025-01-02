import React, { useCallback, useEffect, useState } from 'react';
import { Link } from 'react-router-dom';

import {
  Box,
  Link as ButtonLink,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  Indicator,
  Mark,
  Text,
} from 'design';
import { Danger, Info } from 'design/Alert';
import * as Icons from 'design/Icon';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredAll } from 'shared/components/Validation/rules';
import { useAsync } from 'shared/hooks/useAsync';

import ecfg from 'e-teleport/config';
import { Header } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Shared';
import { usePlugin } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/usePlugin';
import { PluginConfigAwsIc } from 'e-teleport/services/plugins/types';
import { StyledBox } from 'teleport/Discover/Shared';
import { useAwsOidcIntegration } from 'teleport/Integrations/Enroll/AwsOidc/useAwsOidcIntegration';
import {
  RoleArnInput,
  ShowConfigurationScript,
} from 'teleport/Integrations/shared';
import {
  AwsOidcPolicyPreset,
  IntegrationAudience,
  IntegrationKind,
  integrationService,
} from 'teleport/services/integrations';
import useTeleport from 'teleport/useTeleport';

import {
  requiredAwsIdentityCenterInstanceArn,
  requiredAwsIdentityCenterRegion,
  requiredOidcIntegrationName,
  requireUniqueIntegrationName,
} from '../rules';
import { UserAccountWarning } from '../shared/UserAccountWarning';

export function AwsIcOidcIntegration() {
  const { storeUser } = useTeleport();
  const integrationAccess = storeUser.getIntegrationsAccess();
  const hasAccess = integrationAccess.list && integrationAccess.read;
  const { formData, nextStep, setFormData } = usePlugin();
  if (!formData) {
    setFormData(new FormData());
  }
  const {
    integrationConfig,
    setIntegrationConfig,
    scriptUrl,
    setScriptUrl,
    createIntegrationAttempt,
    generateAwsOidcConfigIdpScript,
    runCreateIntegration,
  } = useAwsOidcIntegration();

  const [existingIntegrationName, setExistingIntegrationName] = useState(null);
  const [fetchIntegrationAttempt, runFetchIntegration] = useAsync(
    useCallback(async () => {
      const resp = await integrationService.fetchIntegrations();
      return resp.items
        .map(i => {
          if (i.kind === IntegrationKind.AwsOidc) {
            if (i.spec?.audience === IntegrationAudience.AwsIdentityCenter) {
              setExistingIntegrationName(i.name);
              setIntegrationConfig({
                ...integrationConfig,
                name: i.name,
                roleName: i.name,
              });
            }
            return i.name;
          }
        })
        .filter(Boolean);
    }, [integrationConfig, setIntegrationConfig, setExistingIntegrationName])
  );

  useEffect(() => {
    if (hasAccess && fetchIntegrationAttempt.status === '') {
      runFetchIntegration();
    }
  }, [hasAccess, runFetchIntegration, fetchIntegrationAttempt]);

  const [region, setRegion] = useState(
    formData.get(PluginConfigAwsIc.InstanceRegion)?.toString() || ''
  );
  const [arn, setArn] = useState(
    formData.get(PluginConfigAwsIc.InstanceArn)?.toString() || ''
  );

  async function handleNext(v: Validator) {
    if (!v.validate()) {
      return;
    }

    formData.set(PluginConfigAwsIc.InstanceRegion, region);
    formData.set(PluginConfigAwsIc.InstanceArn, arn);
    formData.set(PluginConfigAwsIc.OidcIntegrationName, integrationConfig.name);

    if (!existingIntegrationName) {
      const [, err] = await runCreateIntegration({
        name: integrationConfig.name,
        subKind: IntegrationKind.AwsOidc,
        awsoidc: {
          roleArn: integrationConfig.roleArn,
          audience: IntegrationAudience.AwsIdentityCenter,
        },
      });
      if (err) {
        return;
      }
    }

    nextStep();
  }

  // AWS IAM Identity Center plugin creates AWS OIDC integration with the
  // same value for integration name and AWS IAM role name.
  function handleNameChange(e: string) {
    setIntegrationConfig({ ...integrationConfig, name: e, roleName: e });
  }

  const nextButtonText = existingIntegrationName
    ? 'Next'
    : 'Save integration and proceed to next step';

  const scriptGenButtonText = scriptUrl
    ? 'Edit'
    : 'Generate Script that configures AWS';

  function scriptGenButtonOnclick(v: Validator) {
    scriptUrl
      ? setScriptUrl('')
      : generateAwsOidcConfigIdpScript(
          v,
          AwsOidcPolicyPreset.AwsIdentityCenter
        );
  }

  return (
    <Box maxWidth="800px">
      <Header header="Configure AWS integration" />
      <Text>
        The integration sets up Teleport as an OIDC IdP for AWS and creates an
        AWS IAM role. Once configured, Teleport AWS IAM Identity Center client
        uses the IAM role to import accounts, user groups, permission sets and
        permission assignments from AWS IAM Identity Center and provision users,
        groups and permission assignments to the AWS IAM Identity Center.
      </Text>
      {/* TODO(sshah): add AWS tagging info once we finalize if we need extra tagging for AWS IAM Identity Center */}
      <Box mt={3}>
        <UserAccountWarning />
      </Box>

      {/* TODO(sshah): add AWS tagging info once we finalize if we need extra tagging for Identity Center */}
      <Box>
        {fetchIntegrationAttempt.status === 'error' && (
          <Danger>{fetchIntegrationAttempt.statusText}</Danger>
        )}
        {existingIntegrationName && (
          <Info>{`OIDC Integration '${existingIntegrationName}' already created for AWS IAM Identity Center plugin. You
        only need to provide the AWS IAM Identity Center instance region and ARN below.`}</Info>
        )}
      </Box>
      {fetchIntegrationAttempt.status === 'processing' ? (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      ) : (
        <Flex
          mb={1}
          mt={4}
          flexDirection="column"
          alignItems="start"
          width="100%"
        >
          <Validation>
            {({ validator }) => (
              <>
                <StyledBox mb={4}>
                  <Text bold>Step 1: Configure AWS Integration</Text>
                  <Text mt={1} mb={4}>
                    AWS IAM Identity Center Region and ARN values can be
                    obtained by navigating to <Mark>Settings &gt; Details</Mark>{' '}
                    in the AWS IAM Identity Center dashboard.
                  </Text>
                  <Flex flexDirection="column" gap={1} mb={4} maxWidth={500}>
                    <FieldInput
                      rule={requiredAwsIdentityCenterRegion}
                      onChange={e => setRegion(e.target.value)}
                      autoFocus={true}
                      label="Enter AWS IAM Identity Center instance region"
                      value={region}
                      placeholder="ca-central-1"
                      toolTipContent={identityCenterRegionToolTip}
                      disabled={!!scriptUrl}
                    />
                    <FieldInput
                      rule={requiredAwsIdentityCenterInstanceArn}
                      onChange={e => setArn(e.target.value)}
                      label="Enter AWS IAM Identity Center instance ARN"
                      value={arn}
                      placeholder="arn:aws:sso:::instance/ssoins-xxxxx"
                      toolTipContent={identityCenterArnToolTip}
                      disabled={!!scriptUrl}
                    />
                    {!existingIntegrationName && (
                      <FieldInput
                        rule={requiredAll(
                          requiredOidcIntegrationName,
                          requireUniqueIntegrationName(
                            fetchIntegrationAttempt.data
                          )
                        )}
                        value={integrationConfig.name}
                        label="Give this AWS OIDC IdP integration a name"
                        placeholder="Integration Name"
                        onChange={e => handleNameChange(e.target.value)}
                        toolTipContent={iamRoleNameToolTip}
                        disabled={!!scriptUrl}
                      />
                    )}
                  </Flex>
                  {!existingIntegrationName && (
                    <ButtonSecondary
                      mb={3}
                      onClick={() => scriptGenButtonOnclick(validator)}
                    >
                      {scriptGenButtonText}
                    </ButtonSecondary>
                  )}
                </StyledBox>
                {scriptUrl && (
                  <>
                    <StyledBox mb={4}>
                      <Text bold>
                        Step 2: Run integration script in AWS Cloud shell.
                      </Text>
                      <ShowConfigurationScript
                        scriptUrl={scriptUrl}
                        description={copyInstallationScriptText}
                      />
                    </StyledBox>
                    <StyledBox mb={5}>
                      <Text bold>Step 3: Enter Role ARN</Text>
                      <RoleArnInput
                        roleName={integrationConfig.name}
                        roleArn={integrationConfig.roleArn}
                        setRoleArn={(v: string) =>
                          setIntegrationConfig({
                            ...integrationConfig,
                            roleArn: v,
                          })
                        }
                        disabled={
                          createIntegrationAttempt.status === 'processing'
                        }
                      />
                    </StyledBox>
                  </>
                )}
                {createIntegrationAttempt.status === 'error' && (
                  <Flex>
                    <Icons.Warning mr={2} color="error.main" size="small" />
                    <Text color="error.main">
                      Error: {createIntegrationAttempt.statusText}
                    </Text>
                  </Flex>
                )}

                <Flex mt={6} mb={5} gap={3}>
                  <ButtonPrimary
                    onClick={() => handleNext(validator)}
                    disabled={
                      !existingIntegrationName &&
                      integrationConfig.roleArn === ''
                    }
                  >
                    {nextButtonText}
                  </ButtonPrimary>

                  <ButtonSecondary
                    as={Link}
                    to={ecfg.oss.getIntegrationEnrollRoute()}
                  >
                    Back
                  </ButtonSecondary>
                </Flex>
              </>
            )}
          </Validation>
        </Flex>
      )}
    </Box>
  );
}

const identityCenterRegionToolTip = `
AWS IAM Identity Center Region value can
be obtained from the AWS IAM Identity Center dashboard by navigating to
"Settings > Details".`;

const identityCenterArnToolTip = `
AWS IAM Identity Center ARN value can
be obtained from AWS IAM Identity Center dashboard by navigating to
"Settings > Details".`;

const iamRoleNameToolTip = `
Teleport will use the name you enter below to create an OIDC
identity provider and an IAM role in AWS with permissions
required for Teleport AWS IAM Identity Center client.`;

const copyInstallationScriptText: React.ReactNode = (
  <>
    <Text>
      Open{' '}
      <ButtonLink
        href="https://console.aws.amazon.com/cloudshell/home"
        target="_blank"
      >
        AWS CloudShell
      </ButtonLink>{' '}
      and copy and paste the bash script shown below.
    </Text>
    <Text mb={2}>
      The script will download and execute Teleport binary that configures
      Teleport as an OIDC identity provider for AWS and creates an IAM role with
      permissions required for Teleport AWS IAM Identity Center client.
    </Text>
  </>
);
