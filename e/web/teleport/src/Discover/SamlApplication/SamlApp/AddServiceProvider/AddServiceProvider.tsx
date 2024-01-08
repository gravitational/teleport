import React, { useState, useEffect } from 'react';
import { Box, Text } from 'design';
import { Danger } from 'design/Alert';
import useAttempt, { State as AttemptState } from 'shared/hooks/useAttemptNext';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import { AgentMeta, useDiscover } from 'teleport/Discover/useDiscover';
import {
  HeaderSubtitle,
  Header,
  ActionButtons,
  StyledBox,
} from 'teleport/Discover/Shared';

import useTeleportE from 'e-teleport/useTeleportE';

import { AddEntityDescriptor } from './EntityDescriptorEditor';
import { AttributeMapping } from './AttributeMapping';

import type {
  AttributeMapping as AttributeMappingType,
  CreateSamlIdpServiceProviderRequest,
} from 'e-teleport/services/idp/types';

export function Container() {
  const { idpService } = useTeleportE();
  const { attempt, run } = useAttempt('');
  const { prevStep, nextStep, updateAgentMeta, agentMeta } = useDiscover();

  const createSP = (spConfig: CreateSamlIdpServiceProviderRequest) => {
    run(() => idpService.createSamlIdpServiceProvider(spConfig));
  };

  const header: React.ReactNode = 'Add Service Provider To Teleport';
  const subtitle: React.ReactNode =
    "Please refer to your Service Provider's documentation for instruction's on how to obtain the Entity ID and ACS URL.";

  return (
    <ServiceProvider
      header={header}
      subtitle={subtitle}
      attempt={attempt}
      createSP={createSP}
      prevStep={prevStep}
      nextStep={nextStep}
      updateAgentMeta={updateAgentMeta}
      agentMeta={agentMeta}
    />
  );
}

/**
 * ServiceProvider is used for adding a generic SAML app as well as specific ones such as Grafana SAML app.
 */
export function ServiceProvider({
  header,
  subtitle,
  attempt,
  createSP,
  agentMeta,
  updateAgentMeta,
  nextStep,
  prevStep,
}: SPProps) {
  const [spConfig, setSPConfig] = useState<CreateSamlIdpServiceProviderRequest>(
    {
      name: '',
      entityID: '',
      acsURL: '',
      entityDescriptor: '',
      attributeMapping: [{ name: '', nameFormat: 'unspecified', value: '' }],
    }
  );

  function validateAndSubmit(
    validator: Validator,
    spConfig: CreateSamlIdpServiceProviderRequest
  ) {
    const spConfigReq = structuredClone(spConfig);
    if (!validator.validate()) {
      return;
    }

    // One empty attribute mapping row is shown. Remove it from
    // request if the values are still empty at this stage.
    if (spConfigReq.attributeMapping.length === 1) {
      if (
        spConfigReq.attributeMapping[0].name.length === 0 &&
        spConfigReq.attributeMapping[0].value.length === 0
      ) {
        spConfigReq.attributeMapping = [];
      }
    }

    // set attribute mapping field error if either attribute name
    // or value is set and the other is not provided.
    for (const attribute of spConfigReq.attributeMapping) {
      if (!checkAndSetAttrMapErr(attribute)) {
        return;
      }
    }

    createSP(spConfigReq);
  }

  // Only one parent field text is shown for each attribute mapping
  // category(name, name format and value). Individual label field is
  // not utilized so empty fields needs to be manually marked as error
  // and LabelInput will be rendered at end of the field with error text.
  const [attrMapErr, setAttrMapErr] = useState({
    emptyName: false,
    emptyValue: false,
  });
  function checkAndSetAttrMapErr(attribute: AttributeMappingType) {
    if (attribute.name.length === 0) {
      setAttrMapErr({ ...attrMapErr, emptyName: true });
      return false;
    }
    if (attribute.value.length === 0) {
      setAttrMapErr({ ...attrMapErr, emptyValue: true });
      return false;
    }
    return true;
  }

  function addAttrMap() {
    if (spConfig.attributeMapping.length > 0) {
      const lastAttrMap =
        spConfig.attributeMapping[spConfig.attributeMapping.length - 1];
      if (!checkAndSetAttrMapErr(lastAttrMap)) {
        return;
      }
    }
    setSPConfig({
      ...spConfig,
      attributeMapping: [
        ...spConfig.attributeMapping,
        { name: '', nameFormat: 'unspecified', value: '' },
      ],
    });
  }

  useEffect(() => {
    if (attempt.status === 'success') {
      updateAgentMeta({ ...agentMeta, resourceName: spConfig.name });
      nextStep();
    }
  }, [attempt, nextStep]);

  return (
    <>
      <Header>{header}</Header>
      <HeaderSubtitle>{subtitle}</HeaderSubtitle>
      {attempt.status === 'failed' && <Danger>{attempt.statusText}</Danger>}
      <Box maxWidth="800px">
        <Validation>
          {({ validator }) => (
            <>
              <SAMLGeneralConfig
                spConfig={spConfig}
                setSPConfig={setSPConfig}
                attempt={attempt}
              />
              <AttributeMapping
                spConfig={spConfig}
                setSPConfig={setSPConfig}
                attrMapErr={attrMapErr}
                setAttrMapErr={setAttrMapErr}
                addAttrMap={addAttrMap}
                attempt={attempt}
              />
              <ActionButtons
                onProceed={() => validateAndSubmit(validator, spConfig)}
                disableProceed={attempt.status === 'processing'}
                onPrev={prevStep}
                lastStep
              />
            </>
          )}
        </Validation>
      </Box>
    </>
  );
}

export type SPProps = {
  header: React.ReactNode;
  subtitle: React.ReactNode;
  attempt: AttemptState['attempt'];
  createSP: (spConfig: CreateSamlIdpServiceProviderRequest) => void;
  agentMeta: AgentMeta;
  updateAgentMeta: (meta: AgentMeta) => void;
  prevStep: () => void;
  nextStep: () => void;
};

export const ErrMissingEntityIDOrACSURL =
  'Either Entity ID and ACS URL or Entity descriptor should be provided';

export function SAMLGeneralConfig({
  setSPConfig,
  spConfig,
  attempt,
}: SAMLGeneralConfig) {
  return (
    <StyledBox>
      <Text bold>Enter the SAML App Service Provider's Metadata</Text>
      <FieldInput
        mb={3}
        rule={requiredField('Name is required')}
        label="App Name"
        autoFocus
        value={spConfig.name}
        placeholder="app_saml"
        width="500px"
        mr="3"
        onChange={e => setSPConfig({ ...spConfig, name: e.target.value })}
        disabled={attempt.status === 'processing'}
      />
      <FieldInput
        mb={3}
        rule={
          !spConfig.entityDescriptor
            ? requiredField(ErrMissingEntityIDOrACSURL)
            : undefined
        }
        label="Entity ID"
        value={spConfig.entityID}
        placeholder="https://example.com/saml/metadata"
        width="500px"
        mr="3"
        onChange={e => setSPConfig({ ...spConfig, entityID: e.target.value })}
        disabled={attempt.status === 'processing'}
      />
      <FieldInput
        mb={3}
        rule={
          !spConfig.entityDescriptor
            ? requiredField(ErrMissingEntityIDOrACSURL)
            : undefined
        }
        label="ACS URL"
        value={spConfig.acsURL}
        placeholder="https://example.com/saml/acs"
        width="500px"
        mr="3"
        onChange={e => setSPConfig({ ...spConfig, acsURL: e.target.value })}
        disabled={attempt.status === 'processing'}
      />
      <AddEntityDescriptor spConfig={spConfig} setSPConfig={setSPConfig} />
    </StyledBox>
  );
}

type SAMLGeneralConfig = {
  setSPConfig: (CreateSamlIdpServiceProviderRequest) => void;
  spConfig: CreateSamlIdpServiceProviderRequest;
  attempt: AttemptState['attempt'];
};
