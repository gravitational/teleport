import React, { useEffect, useState } from 'react';

import { Box, ButtonBorder, Flex, Link, Text, Toggle } from 'design';
import { IconTooltip } from 'design/Tooltip';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import {
  requiredAll,
  requiredField,
  Rule,
} from 'shared/components/Validation/rules';

import { IdpMetadata } from 'e-teleport/SamlApplication/components/IdpMetadata';
import {
  checkDefaultAttributePerPreset,
  genEntityIDAndAcsUrlForGcpWorkforce,
  useSamlApplication,
} from 'e-teleport/SamlApplication/hooks/useSamlApplication';
import { gcpWorkforcePresetSpec } from 'e-teleport/services/idp/types';
import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';
import cfg from 'teleport/config';
import {
  ActionButtons,
  Header,
  HeaderSubtitle,
  StyledBox,
} from 'teleport/Discover/Shared';
import { SamlMeta, useDiscover } from 'teleport/Discover/useDiscover';
import { SamlServiceProviderPreset } from 'teleport/services/samlidp/types';

export function Container() {
  const { prevStep, nextStep, isUpdateFlow, agentMeta } = useDiscover();
  const {
    guidedToggle,
    setGuidedToggle,
    upsertRequest,
    setUpsertRequest,
    guidedConfig,
    setGuidedConfig,
  } = useSamlApplication();

  // value of agentMeta will be defined if user is coming to
  // this screen from update Discover flow or coming back from the
  // next screen. But it's value will be undefined if the user is coming
  // to this screen from the "Enroll New Resource" Discover screen.
  const samlMeta: SamlMeta = agentMeta
    ? agentMeta
    : defaultSamlMetaForGcpWorkforce;

  useEffect(() => {
    if (guidedToggle == null) {
      if (!isUpdateFlow) {
        setGuidedToggle(true);
        setGuidedConfig(defaultSamlMetaForGcpWorkforce);
      } else {
        // guidedToggle disabeld by default on update.
        // TODO(sshah): Disable guided flow entirely on update.
        // Because the user may update pool provider name in the GCP and since
        // the pool provider name is also used as a SAML app name, and we
        // do not allow updating app name during update, it is easy to
        // make the configuration in GCP and Teleport go out of sync.
        setGuidedConfig({ samlGcpWorkforce: samlMeta.samlGcpWorkforce });
      }
    }
  }, [guidedToggle, setGuidedToggle, setGuidedConfig, isUpdateFlow, samlMeta]);

  const handleNext = (validator: Validator) => {
    if (guidedToggle) {
      const entityIdAndAcsUrl = genEntityIDAndAcsUrlForGcpWorkforce(
        guidedConfig.samlGcpWorkforce.poolName,
        guidedConfig.samlGcpWorkforce.poolProviderName
      );

      // we don't set default attribute value in update flow as user may have previously
      // opted for manual configuration.
      const defaultAttribute =
        !isUpdateFlow &&
        checkDefaultAttributePerPreset(
          SamlServiceProviderPreset.GcpWorkforce,
          upsertRequest.attributeMapping
        )
          ? upsertRequest.attributeMapping
          : gcpWorkforcePresetSpec().attribute_mapping;

      setUpsertRequest({
        ...upsertRequest,
        name: isUpdateFlow
          ? upsertRequest.name
          : guidedConfig.samlGcpWorkforce.poolProviderName,
        entityID: entityIdAndAcsUrl.entityId,
        acsURL: entityIdAndAcsUrl.acsUrl,
        preset: SamlServiceProviderPreset.GcpWorkforce,
        attributeMapping: defaultAttribute,
      });
    }

    if (!validator.validate()) {
      return;
    }

    nextStep();
  };

  return (
    <ConfigurePool
      /**
       * In an update flow, user's should be prevent from navigating to
       * the root Discover resource selection page.
       */
      prevStep={isUpdateFlow ? null : prevStep}
      nextStep={handleNext}
    />
  );
}

export const defaultSamlMetaForGcpWorkforce: SamlMeta = {
  samlGcpWorkforce: {
    orgId: '',
    poolName: '',
    poolProviderName: '',
  },
};

export type ConfigurePoolProps = {
  nextStep: (validator: Validator) => void;
  prevStep: () => void;
};

export function ConfigurePool({ prevStep, nextStep }: ConfigurePoolProps) {
  const { guidedToggle, setGuidedToggle, guidedConfig } = useSamlApplication();

  const [scriptUrl, setScriptUrl] = useState('');
  function genWorkforceConfigScript(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    validator.reset();

    const newScriptUrl = cfg.getGcpWorkforceConfigScriptUrl({
      orgId: guidedConfig.samlGcpWorkforce.orgId,
      poolName: guidedConfig.samlGcpWorkforce.poolName,
      poolProviderName: guidedConfig.samlGcpWorkforce.poolProviderName,
    });

    setScriptUrl(newScriptUrl);
  }

  function handleConfigModeChange() {
    setGuidedToggle(!guidedToggle);
  }
  return (
    <>
      <Header>
        Configure GCP Workforce Pool with Teleport's Identity Provider Metadata
      </Header>
      <HeaderSubtitle>
        You can choose between a guided or a manual flow. With the guided flow,
        Teleport generates a GCP Workforce <br /> Identity Federation
        configuration script and pre-populates SAML service provider spec based
        on GCP configuration <br /> values you enter below.
      </HeaderSubtitle>
      <GCPPrerequisites />
      <Box mt={6} mb={1} data-testid="testid-box">
        <Toggle
          className="toggle_test"
          isToggled={!!guidedToggle}
          onToggle={handleConfigModeChange}
          data-testid="toggle_test"
        >
          <Text ml={2}>
            Guided configuration flow is {guidedToggle ? 'enabled' : 'disabled'}
            .
          </Text>
        </Toggle>
      </Box>
      <Validation>
        {({ validator }) => (
          <>
            {guidedToggle ? (
              <>
                <ScriptGenInput
                  genWorkforceConfigScript={genWorkforceConfigScript}
                  validator={validator}
                />
                {scriptUrl && <Script scriptUrl={scriptUrl} />}
              </>
            ) : (
              <IdpMetadata />
            )}
            <ActionButtons
              onProceed={() => nextStep(validator)}
              onPrev={prevStep}
            />
          </>
        )}
      </Validation>
    </>
  );
}

type ScriptGenPropTypes = {
  genWorkforceConfigScript: (validator: Validator) => void;
  validator: Validator;
};

function ScriptGenInput({
  genWorkforceConfigScript,
  validator,
}: ScriptGenPropTypes) {
  const { guidedConfig, setGuidedConfig } = useSamlApplication();
  function handleNameChange(e: React.ChangeEvent<HTMLInputElement>) {
    setGuidedConfig({
      ...guidedConfig,
      samlGcpWorkforce: {
        ...guidedConfig.samlGcpWorkforce,
        [e.target.name]: e.target.value,
      },
    });
  }
  return (
    <StyledBox>
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
        value={guidedConfig.samlGcpWorkforce.orgId}
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
        value={guidedConfig.samlGcpWorkforce.poolName}
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
        value={guidedConfig.samlGcpWorkforce.poolProviderName}
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
    </StyledBox>
  );
}

function Script({ scriptUrl }: { scriptUrl: string }) {
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
                <IconTooltip>Always assign least privileged roles.</IconTooltip>
              </Flex>
            </Flex>
          </li>
        </ul>
      </Flex>
    </>
  );
}
