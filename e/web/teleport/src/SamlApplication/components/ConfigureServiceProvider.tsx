import React, { useEffect, useState } from 'react';

import { Box, Text } from 'design';
import { Danger } from 'design/Alert';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import {
  transformSamlSpecToCreateRequest,
  useSamlApplication,
} from 'e-teleport/SamlApplication/hooks/useSamlApplication';
import type { CreateSamlIdpServiceProviderRequest } from 'e-teleport/services/idp/types';
import {
  ActionButtons,
  Header,
  HeaderSubtitle,
  StyledBox,
} from 'teleport/Discover/Shared';
import type { AgentMeta } from 'teleport/Discover/useDiscover';
import {
  AttributeMapping as AttributeMappingType,
  SamlServiceProviderPreset,
} from 'teleport/services/samlidp/types';

import { AttributeMapping } from './AttributeMapping';
import { AddEntityDescriptor } from './EntityDescriptorEditor';

/**
 * ConfigureServiceProvider is used for adding a generic SAML app as well as specific ones such as Grafana SAML app.
 */
export function ConfigureServiceProvider({
  header,
  subtitle,
  agentMeta,
  updateAgentMeta,
  nextStep,
  prevStep,
  SpMetadataConfigComponent,
  isUpdateFlow,
  preset,
}: ConfigureServiceProviderProps) {
  const {
    upsertRequest,
    setUpsertRequest,
    runUpsert,
    upsertAttempt,
    guidedToggle,
  } = useSamlApplication();

  useEffect(() => {
    if (!upsertRequest.name) {
      if (agentMeta && isUpdateFlow) {
        setUpsertRequest(transformSamlSpecToCreateRequest(agentMeta));
      }
      if (upsertRequest.preset != preset) {
        setUpsertRequest({
          ...upsertRequest,
          preset: preset,
        });
      }
    }
  }, [agentMeta, isUpdateFlow, upsertRequest, setUpsertRequest, preset]);

  const upsert = async (spConfig: CreateSamlIdpServiceProviderRequest) => {
    const [, err] = await runUpsert(spConfig, isUpdateFlow);
    if (err) {
      return;
    }
    updateAgentMeta({
      ...agentMeta,
      resourceName: upsertRequest.name,
    });
    nextStep();
  };

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

    upsert(spConfigReq);
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
    if (upsertRequest.attributeMapping.length > 0) {
      const lastAttrMap =
        upsertRequest.attributeMapping[
          upsertRequest.attributeMapping.length - 1
        ];
      if (!checkAndSetAttrMapErr(lastAttrMap)) {
        return;
      }
    }
    setUpsertRequest({
      ...upsertRequest,
      attributeMapping: [
        ...upsertRequest.attributeMapping,
        { name: '', name_format: 'unspecified', value: '' },
      ],
    });
  }

  return (
    <>
      <Header>{header}</Header>
      <HeaderSubtitle>{subtitle}</HeaderSubtitle>
      {upsertAttempt.status === 'error' && (
        <Danger>{upsertAttempt.statusText}</Danger>
      )}
      <Box maxWidth="800px">
        <Validation>
          {({ validator }) => (
            <>
              <SpMetadataConfigComponent
                spConfig={upsertRequest}
                setSPConfig={setUpsertRequest}
                isUpdateFlow={isUpdateFlow}
                disableInputs={
                  upsertAttempt.status === 'processing' || guidedToggle
                }
              />
              <AttributeMapping
                spConfig={upsertRequest}
                setSPConfig={setUpsertRequest}
                attrMapErr={attrMapErr}
                setAttrMapErr={setAttrMapErr}
                addAttrMap={addAttrMap}
                disabled={upsertAttempt.status === 'processing'}
                preset={preset}
                isGuided={guidedToggle}
              />
              <ActionButtons
                onProceed={() => validateAndSubmit(validator, upsertRequest)}
                disableProceed={upsertAttempt.status === 'processing'}
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

export function AddMetadataGeneric({
  setSPConfig,
  spConfig,
  disableInputs = false,
  isUpdateFlow = false,
}: SamlGenericMetadataConfig) {
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
        disabled={disableInputs || isUpdateFlow}
      />
      <FieldInput
        mb={3}
        rule={
          !spConfig.entityDescriptor
            ? requiredField(ErrMissingEntityIDOrACSURL)
            : undefined
        }
        label="SP Entity ID / Audience URI"
        toolTipContent="SP Entity ID is a unique identifier of the service provider.
        This is also commonly referred to as an Audience URI."
        value={spConfig.entityID}
        placeholder="https://example.com/saml/metadata"
        width="500px"
        mr="3"
        onChange={e => setSPConfig({ ...spConfig, entityID: e.target.value })}
        disabled={disableInputs}
      />
      <FieldInput
        mb={3}
        rule={
          !spConfig.entityDescriptor
            ? requiredField(ErrMissingEntityIDOrACSURL)
            : undefined
        }
        label="ACS URL / SP SSO URL"
        toolTipContent="Assertion Consumer Service (ACS) URL is a location where users will be redirected after authenticating with the IdP.
        This is also commonly referred to as the Single Sign-on (SSO) URL."
        value={spConfig.acsURL}
        placeholder="https://example.com/saml/acs"
        width="500px"
        mr="3"
        onChange={e => setSPConfig({ ...spConfig, acsURL: e.target.value })}
        disabled={disableInputs}
      />
      {!disableInputs && (
        <AddEntityDescriptor spConfig={spConfig} setSPConfig={setSPConfig} />
      )}
    </StyledBox>
  );
}

export type ConfigureServiceProviderProps = {
  header: React.ReactNode;
  subtitle: React.ReactNode;
  agentMeta: AgentMeta;
  updateAgentMeta: (meta: AgentMeta) => void;
  prevStep: () => void;
  nextStep: () => void;
  SpMetadataConfigComponent: (props: SamlGenericMetadataConfig) => JSX.Element;
  isUpdateFlow: boolean;
  preset?: SamlServiceProviderPreset;
};

export const ErrMissingEntityIDOrACSURL =
  'Either Entity ID and ACS URL or Entity descriptor should be provided';

export type SamlGenericMetadataConfig = {
  setSPConfig: (params: CreateSamlIdpServiceProviderRequest) => void;
  spConfig: CreateSamlIdpServiceProviderRequest;
  isUpdateFlow: boolean;
  disableInputs: boolean;
};

export const UPDATE_NOTE =
  '*To update an Entity ID and ACS URL, both the input fields and the entity descriptor must be updated.';
