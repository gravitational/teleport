import React, { useEffect, useState } from 'react';
import { Box, Text } from 'design';
import { Danger } from 'design/Alert';

import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import { State as AttemptState } from 'shared/hooks/useAttemptNext';

import {
  ActionButtons,
  Header,
  HeaderSubtitle,
  StyledBox,
} from 'teleport/Discover/Shared';

import FieldInput from 'shared/components/FieldInput';

import { AgentMeta } from 'teleport/Discover/useDiscover';
import { type ResourceSpec } from 'teleport/Discover/SelectResource/types';

import { SamlServiceProviderPreset } from 'teleport/services/samlidp/types';

import { AttributeMapping } from './AttributeMapping';
import { AddEntityDescriptor } from './EntityDescriptorEditor';

import type {
  AttributeMapping as AttributeMappingType,
  SamlGcpWorkforce,
} from 'teleport/services/samlidp/types';

import type { CreateSamlIdpServiceProviderRequest } from 'e-teleport/services/idp/types';

/**
 * ConfigureServiceProvider is used for adding a generic SAML app as well as specific ones such as Grafana SAML app.
 */
export function ConfigureServiceProvider({
  header,
  subtitle,
  attempt,
  createSP,
  agentMeta,
  updateAgentMeta,
  nextStep,
  prevStep,
  SpMetadataConfigComponent,
  resourceSpec,
}: ConfigureServiceProviderProps) {
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
    if (
      resourceSpec?.samlMeta?.preset === SamlServiceProviderPreset.GcpWorkforce
    ) {
      const gcpWorkforceMeta = agentMeta as SamlGcpWorkforce;
      if (gcpWorkforceMeta?.isAutoConfig) {
        const entityIdAndAcsUrl = genEntityIDAndAcsUrlForGcpWorkforce(
          gcpWorkforceMeta.poolName,
          gcpWorkforceMeta.poolProviderName
        );
        setSPConfig({
          ...spConfig,
          name: gcpWorkforceMeta?.poolProviderName,
          entityID: entityIdAndAcsUrl.entityId,
          acsURL: entityIdAndAcsUrl.acsUrl,
          attributeMapping: [
            {
              name: 'roles',
              nameFormat: 'unspecified',
              value: 'user.spec.roles',
            },
          ],
        });
      }
    }
  }, [agentMeta, resourceSpec]);

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
              <SpMetadataConfigComponent
                spConfig={spConfig}
                setSPConfig={setSPConfig}
                attempt={attempt}
                agentMeta={agentMeta}
                resourceSpec={resourceSpec}
              />
              <AttributeMapping
                spConfig={spConfig}
                setSPConfig={setSPConfig}
                attrMapErr={attrMapErr}
                setAttrMapErr={setAttrMapErr}
                addAttrMap={addAttrMap}
                attempt={attempt}
                agentMeta={agentMeta}
                resourceSpec={resourceSpec}
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

export function AddMetadataGeneric({
  setSPConfig,
  spConfig,
  attempt,
  agentMeta,
  resourceSpec,
}: SamlGenericMetadataConfig) {
  const [disabled, setDisabled] = useState(false);
  useEffect(() => {
    if (
      resourceSpec?.samlMeta?.preset === SamlServiceProviderPreset.GcpWorkforce
    ) {
      const gcpWorkforceMeta = agentMeta as SamlGcpWorkforce;
      if (gcpWorkforceMeta?.isAutoConfig) {
        setDisabled(true);
        setSPConfig({ ...spConfig, name: gcpWorkforceMeta?.poolProviderName });
      }
    }
  }, []);

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
        disabled={attempt.status === 'processing' || disabled}
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
        disabled={attempt.status === 'processing' || disabled}
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
        disabled={attempt.status === 'processing' || disabled}
      />
      {!disabled && (
        <AddEntityDescriptor spConfig={spConfig} setSPConfig={setSPConfig} />
      )}
    </StyledBox>
  );
}

export type ConfigureServiceProviderProps = {
  header: React.ReactNode;
  subtitle: React.ReactNode;
  attempt: AttemptState['attempt'];
  createSP: (spConfig: CreateSamlIdpServiceProviderRequest) => void;
  agentMeta: AgentMeta;
  updateAgentMeta: (meta: AgentMeta) => void;
  prevStep: () => void;
  nextStep: () => void;
  SpMetadataConfigComponent: (props: SamlGenericMetadataConfig) => JSX.Element;
  resourceSpec?: ResourceSpec;
};

export const ErrMissingEntityIDOrACSURL =
  'Either Entity ID and ACS URL or Entity descriptor should be provided';

export type SamlGenericMetadataConfig = {
  setSPConfig: (CreateSamlIdpServiceProviderRequest) => void;
  spConfig: CreateSamlIdpServiceProviderRequest;
  attempt: AttemptState['attempt'];
  agentMeta?: AgentMeta;
  resourceSpec: ResourceSpec;
};

export function genEntityIDAndAcsUrlForGcpWorkforce(
  poolName: string,
  poolProviderName: string
) {
  const entityId = `https://iam.googleapis.com/locations/global/workforcePools/${poolName}/providers/${poolProviderName}`;
  const acsUrl = `https://auth.cloud.google/signin-callback/locations/global/workforcePools/${poolName}/providers/${poolProviderName}`;
  return { entityId, acsUrl };
}
