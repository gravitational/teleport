import React, { useState, useEffect } from 'react';

import { Box, ButtonBorder, Link, Text, Toggle, Flex } from 'design';

import { Danger } from 'design/Alert';

import cfg from 'teleport/config';

import {
  ActionButtons,
  Header,
  HeaderSubtitle,
  StyledBox,
} from 'teleport/Discover/Shared';

import FieldInput from 'shared/components/FieldInput';
import { ToolTipInfo } from 'shared/components/ToolTip';

import Validation, { Validator } from 'shared/components/Validation';
import {
  requiredAll,
  requiredField,
  Rule,
} from 'shared/components/Validation/rules';

import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';

import { useAttemptNext } from 'shared/hooks';

import {
  useDiscover,
  AgentMeta,
  type SamlMeta,
} from 'teleport/Discover/useDiscover';

import useTeleportE from 'e-teleport/useTeleportE';

import { ConfigureServiceProvider } from 'e-teleport/Discover/SamlApplication/Generic/DownloadMetadata/DownloadMetadata';

import type { SAMLIdPMetadataResponse } from 'e-teleport/services/idp/types';

export function Container() {
  const { prevStep, nextStep, agentMeta, updateAgentMeta, isUpdateFlow } =
    useDiscover();
  // value of agentMeta will be defined if user is coming to
  // this screen from update Discover flow or coming back from the
  // next screen. But it's value will be undefined if the user is coming
  // to this screen from the "Enroll New Resource" Discover screen.
  const samlMeta: SamlMeta = agentMeta
    ? agentMeta
    : defaultSamlMetaForGcpWorkforce;

  const { idpService } = useTeleportE();

  return (
    <ConfigurePool
      /**
       * In an update flow, user's should be prevent from navigating to
       * the root Discover resource selection page.
       */
      prevStep={isUpdateFlow ? null : prevStep}
      nextStep={nextStep}
      fetchMetadata={idpService.getIdPMetadataValues}
      agentMeta={samlMeta}
      updateAgentMeta={updateAgentMeta}
    />
  );
}

export const defaultSamlMetaForGcpWorkforce: SamlMeta = {
  samlGcpWorkforce: {
    isAutoConfig: true,
    orgId: '',
    poolName: '',
    poolProviderName: '',
  },
};

export type ConfigurePoolProps = {
  nextStep: () => void;
  prevStep: () => void;
  fetchMetadata: () => Promise<SAMLIdPMetadataResponse>;
  agentMeta: SamlMeta;
  updateAgentMeta?: (meta: AgentMeta) => void;
};

export function ConfigurePool({
  prevStep,
  nextStep,
  agentMeta,
  updateAgentMeta,
  fetchMetadata,
}: ConfigurePoolProps) {
  const { attempt, setAttempt } = useAttemptNext('processing');
  const [samlIdPMetadata, setSAMLIdPMetadata] =
    useState<SAMLIdPMetadataResponse>({
      entityID: '',
      ssoURL: '',
      x509PEM: '',
    });

  const [autoConfig, setAutoConfig] = useState<boolean>(
    agentMeta?.samlGcpWorkforce?.isAutoConfig
  );

  useEffect(() => {
    updateAgentMeta({
      ...agentMeta,
      samlGcpWorkforce: {
        ...agentMeta.samlGcpWorkforce,
        isAutoConfig: autoConfig,
      },
    });
  }, [autoConfig]);

  useEffect(() => {
    if (!autoConfig) {
      fetchMetadata()
        .then(resp => {
          setAttempt({ status: 'success' });
          setSAMLIdPMetadata(resp);
        })
        .catch((err: Error) => {
          setAttempt({ status: 'failed', statusText: err.message });
        });
    }
  }, [autoConfig]);

  const [scriptUrl, setScriptUrl] = useState('');
  function genWorkforceConfigScript(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    validator.reset();

    const newScriptUrl = cfg.getGcpWorkforceConfigScriptUrl({
      orgId: agentMeta.samlGcpWorkforce.orgId,
      poolName: agentMeta.samlGcpWorkforce.poolName,
      poolProviderName: agentMeta.samlGcpWorkforce.poolProviderName,
    });

    setScriptUrl(newScriptUrl);
  }

  function handleConfigModeChange() {
    setAutoConfig(!autoConfig);
    updateAgentMeta({
      ...agentMeta,
      samlGcpWorkforce: {
        ...agentMeta.samlGcpWorkforce,
        isAutoConfig: !autoConfig,
      },
    });
  }
  return (
    <>
      <Header>
        Configure GCP Workforce Pool with Teleport's Identity Provider Metadata
      </Header>
      <HeaderSubtitle>
        You can choose between a guided or a manual flow. With the guided flow,
        Teleport generates a GCP Workforce <br /> Identity Federation
        configuration script and pre-pulates SAML service provider spec based on
        GCP configuration <br /> values you enter below.
      </HeaderSubtitle>
      <GCPPrerequisites />
      <Box mt={6} mb={1} data-testid="testid-box">
        <Toggle
          className="toggle_test"
          isToggled={autoConfig}
          onToggle={handleConfigModeChange}
          data-testid="toggle_test"
        >
          <Text ml={2}>
            Guided configuration flow is {autoConfig ? 'enabled' : 'disabled'}.
          </Text>
        </Toggle>
      </Box>

      {attempt.status === 'failed' && <Danger>{attempt.statusText}</Danger>}

      {autoConfig ? (
        <>
          <ScriptGenInput
            genWorkforceConfigScript={genWorkforceConfigScript}
            agentMeta={agentMeta}
            updateAgentMeta={updateAgentMeta}
          />
          {scriptUrl && <Script scriptUrl={scriptUrl} />}
        </>
      ) : (
        <ConfigureServiceProvider samlIdPMetadata={samlIdPMetadata} />
      )}
      <ActionButtons onProceed={nextStep} onPrev={prevStep} />
    </>
  );
}

type ScriptGenPropTypes = {
  genWorkforceConfigScript: (validator: Validator) => void;
  agentMeta?: SamlMeta;
  updateAgentMeta?: (meta: AgentMeta) => void;
};

export function ScriptGenInput({
  genWorkforceConfigScript,
  agentMeta,
  updateAgentMeta,
}: ScriptGenPropTypes) {
  function handleNameChange(e: React.ChangeEvent<HTMLInputElement>) {
    updateAgentMeta({
      ...agentMeta,
      samlGcpWorkforce: {
        ...agentMeta.samlGcpWorkforce,
        [e.target.name]: e.target.value,
      },
    });
  }
  return (
    <StyledBox>
      <Validation>
        {({ validator }) => (
          <>
            <Text bold>Step 1:</Text>
            Generate an installation command to configure Workforce Identity
            Federation pool and pool provider
            <FieldInput
              mb={3}
              rule={requiredAll(
                requiredField('Organization ID is required'),
                isValidGcpOrgID
              )}
              label="GCP organization ID"
              toolTipContent="Obtain organization ID from GCP console."
              autoFocus
              name="orgId"
              value={agentMeta.samlGcpWorkforce?.orgId}
              placeholder="10xxxxxxxxx44"
              width="500px"
              mr="3"
              onChange={handleNameChange}
            />
            <FieldInput
              mb={3}
              rule={requiredAll(
                requiredField('Pool name is required'),
                isValidGCPResourceName
              )}
              label="Workforce pool name"
              toolTipContent="Pool name you want to configure in GCP. Name must be a unique name
              across GCP and follow GCP resource naming convention."
              name="poolName"
              value={agentMeta.samlGcpWorkforce?.poolName}
              placeholder="myorg-workforce-dev-pool"
              width="500px"
              mr="3"
              onChange={e => handleNameChange(e)}
            />
            <FieldInput
              mb={3}
              rule={requiredAll(
                requiredField('Pool provider name is required'),
                isValidGCPResourceName
              )}
              label="App name - Workforce pool provider name"
              toolTipContent="Pool provider name you want to configure in GCP. Name must be a unique
              name across GCP and follow GCP resource naming convention. Pool provider name will also
              be used as a SAML service provider name in the next step."
              name="poolProviderName"
              value={agentMeta.samlGcpWorkforce?.poolProviderName}
              placeholder="myorg-gcp-dev"
              width="500px"
              mr="3"
              onChange={e => handleNameChange(e)}
            />
            <ButtonBorder
              mt={3}
              mb={3}
              onClick={() => genWorkforceConfigScript(validator)}
            >
              Generate Command
            </ButtonBorder>
          </>
        )}
      </Validation>
    </StyledBox>
  );
}

export const ErrGcpOrgId = 'GCP organization ID should be numeric value';

export function Script({ scriptUrl }: { scriptUrl: string }) {
  return (
    <StyledBox mb={5} mt={5} data-testid="scriptbox">
      <Text bold>Step 2:</Text>
      Configure Workforce Identity Federation pool in your GCP account.
      <Text mb={2}>
        Open{' '}
        <Link
          href="https://shell.cloud.google.com/?show=terminal"
          target="_blank"
        >
          GCP CloudShell
        </Link>{' '}
        and copy and paste the installation command shown below that configures
        the Workforce Identity Federation based on the input provided above.
      </Text>
      <Box mb={2}>
        <TextSelectCopyMulti
          lines={[
            {
              text: `bash -c "$(curl '${scriptUrl}')"`,
            },
          ]}
        />
      </Box>
    </StyledBox>
  );
}

/**
 * isValidGcpOrgID validates GCP organization ID, which
 * should be numeric only value.
 */
export const isValidGcpOrgID: Rule = value => () => {
  if (isNaN(value as any)) {
    return {
      valid: false,
      message: 'GCP organization ID must be a numeric value',
    };
  }

  return {
    valid: true,
  };
};

/**
 * isValidGCPResourceName validates name based on GCP naming convention.
 * https://cloud.google.com/compute/docs/naming-resources#resource-name-format.
 */
export const isValidGCPResourceName: Rule = value => () => {
  if (value && value.length > 63) {
    return {
      valid: false,
      message: 'Name cannot exceed 63 character length',
    };
  }
  const resourceRegex = new RegExp('^[a-z]([-a-z0-9]*[a-z0-9])?$');
  if (value && !resourceRegex.test(value)) {
    return {
      valid: false,
      message: 'Name does not follow GCP resource naming convention',
    };
  }

  return {
    valid: true,
  };
};

function GCPPrerequisites() {
  return (
    <>
      <Text fontSize={2} bold>
        Prerequisites:
      </Text>
      <Flex gap={6} bg="levels.surface" borderRadius={2} mt={1} mb={4}>
        <ul>
          <li>
            <Flex alignItems="center">
              <Text>
                IAM and Resource Manager APIs enabled for organization{' '}
                <Link
                  target="_blank"
                  href="https://cloud.google.com/iam/docs/configuring-workforce-identity-federation#before_you_begin"
                >
                  (docs).
                </Link>
              </Text>
            </Flex>
          </li>
          <li>
            <Flex alignItems="center">
              <Text>
                User with IAM Workforce Pool Admin and Organization Viewer role{' '}
                <Link
                  target="_blank"
                  href="https://cloud.google.com/iam/docs/configuring-workforce-identity-federation#required-roles"
                >
                  (docs).
                </Link>
              </Text>
              <Flex ml={1}>
                <ToolTipInfo>Always assign least privileged roles.</ToolTipInfo>
              </Flex>
            </Flex>
          </li>
        </ul>
      </Flex>
    </>
  );
}
