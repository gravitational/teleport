import React, { useCallback, useEffect, useState } from 'react';
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
import * as Icons from 'design/Icon';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredAll } from 'shared/components/Validation/rules';
import { useAsync } from 'shared/hooks/useAsync';
import ErrorMessage from 'teleport/components/AgentErrorMessage';
import { StyledBox } from 'teleport/Discover/Shared';
import { useAwsOidcIntegration } from 'teleport/Integrations/Enroll/AwsOidc/useAwsOidcIntegration';
import {
  RoleArnInput,
  ShowConfigurationScript,
} from 'teleport/Integrations/shared';
import {
  IntegrationKind,
  integrationService,
  AwsOidcPolicyPreset,
} from 'teleport/services/integrations';
import useTeleport from 'teleport/useTeleport';

import { Header } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Shared';

import {
  requiredOidcIntegrationName,
  requireUniqueIntegrationName,
  requiredAwsIdentityCenterInstanceArn,
  requiredAwsIdentityCenterRegion,
} from '../rules';

export function AwsIcOidcIntegration() {
  const { storeUser } = useTeleport();
  const integrationAccess = storeUser.getIntegrationsAccess();
  const hasAccess = integrationAccess.list && integrationAccess.read;
  const {
    integrationConfig,
    setIntegrationConfig,
    scriptUrl,
    setScriptUrl,
    createIntegrationAttempt,
    generateAwsOidcConfigIdpScript,
  } = useAwsOidcIntegration();

  const [fetchIntegrationAttempt, runFetchIntegration] = useAsync(
    useCallback(async () => {
      const resp = await integrationService.fetchIntegrations();
      return resp.items
        .map(i => {
          if (i.kind === IntegrationKind.AwsOidc) {
            return i.name;
          }
        })
        .filter(Boolean);
    }, [])
  );

  useEffect(() => {
    if (hasAccess && fetchIntegrationAttempt.status === '') {
      runFetchIntegration();
    }
  }, [hasAccess, runFetchIntegration, fetchIntegrationAttempt]);

  const [region, setRegion] = useState('');
  const [arn, setArn] = useState('');

  function handleNext(v: Validator) {
    if (!v.validate()) {
      return;
    }

    // TODO(sshah): check if integration already exist or create a new one
    // and move to next step
  }

  // AWS Identity Center plugin creates AWS OIDC integration with the
  // same value for integration name and AWS IAM role name.
  function handleRoleNameChange(e: string) {
    setIntegrationConfig({ ...integrationConfig, name: e, roleName: e });
  }

  const nextButtonText = fetchIntegrationAttempt.data?.includes(
    integrationConfig.name
  )
    ? 'Next'
    : 'Save integration and proceed to next step';

  return (
    <Box maxWidth="800px">
      <Header header="Configure AWS integration" />
      <Text>
        The integration sets up Teleport as an OIDC identity provider for AWS
        and creates an AWS role. Once configured, Teleport Identity Center
        client uses the role to import Identity Center accounts, user groups and
        permission sets from Identity Center and provision users, groups and
        permission assignments to the Identity Center.
      </Text>
      {/* TODO(sshah): add AWS tagging info once we finalize if we need extra tagging for Identity Center */}
      {fetchIntegrationAttempt.status === 'error' && (
        <Box mt={3}>
          <ErrorMessage message={fetchIntegrationAttempt.statusText} />
        </Box>
      )}

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
                    Identity Center Region and ARN values can be obtained by
                    navigating to <Mark>Settings &gt; Details</Mark> in the
                    Identity Center dashboard.
                  </Text>
                  <Flex flexDirection="column" gap={1} mb={4} maxWidth={500}>
                    <FieldInput
                      rule={requiredAwsIdentityCenterRegion}
                      onChange={e => setRegion(e.target.value)}
                      autoFocus={true}
                      label="Enter Identity Center region"
                      value={region}
                      placeholder="ca-central-1"
                      toolTipContent={identityCenterRegionToolTip}
                      disabled={!!scriptUrl}
                    />
                    <FieldInput
                      rule={requiredAwsIdentityCenterInstanceArn}
                      onChange={e => setArn(e.target.value)}
                      label="Enter Identity Center ARN"
                      value={arn}
                      placeholder="arn:aws:sso:::instance/ssoins-xxxxx"
                      toolTipContent={identityCenterArnToolTip}
                      disabled={!!scriptUrl}
                    />
                    <FieldInput
                      rule={requiredAll(
                        requiredOidcIntegrationName,
                        requireUniqueIntegrationName(
                          fetchIntegrationAttempt.data
                        )
                      )}
                      value={integrationConfig.name}
                      label="Give this AWS integration a name"
                      placeholder="Integration Name"
                      onChange={e => handleRoleNameChange(e.target.value)}
                      toolTipContent={iamRoleNameToolTip}
                      disabled={!!scriptUrl}
                    />
                  </Flex>
                  {scriptUrl ? (
                    <ButtonSecondary
                      mb={3}
                      onClick={() => {
                        setScriptUrl('');
                      }}
                    >
                      Edit
                    </ButtonSecondary>
                  ) : (
                    <ButtonSecondary
                      mb={3}
                      onClick={() =>
                        generateAwsOidcConfigIdpScript(
                          validator,
                          AwsOidcPolicyPreset.AwsIdentityCenter
                        )
                      }
                    >
                      Generate Script that configures AWS
                    </ButtonSecondary>
                  )}
                </StyledBox>
                {scriptUrl && (
                  <StyledBox mb={4}>
                    <Text bold>
                      Step 2: Run integration script in AWS Cloud shell.
                    </Text>
                    <ShowConfigurationScript
                      scriptUrl={scriptUrl}
                      description={copyInstallationScriptText}
                    />
                  </StyledBox>
                )}
                {scriptUrl && (
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
                    disabled={integrationConfig.roleArn === ''}
                  >
                    {nextButtonText}
                  </ButtonPrimary>

                  <ButtonSecondary>Back</ButtonSecondary>
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
Identity Center Region value can
be obtained from the Identity Center dashboard by navigating to
"Settings > Details".`;

const identityCenterArnToolTip = `
Identity Center ARN value can
be obtained from Identity Center dashboard by navigating to
"Settings > Details".`;

const iamRoleNameToolTip = `
Teleport will use the name you enter below to create an OIDC
identity provider and an IAM role in AWS with permissions
required for Teleport Identity Center client.`;

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
      permissions required for Teleport Identity Center client.
    </Text>
  </>
);
