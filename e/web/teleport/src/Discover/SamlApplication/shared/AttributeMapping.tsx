import React from 'react';
import { Box, ButtonIcon, Flex, Text, LabelInput, Link } from 'design';
import * as Icons from 'design/Icon';
import FieldInput from 'shared/components/FieldInput';
import FieldSelect, {
  FieldSelectCreatable,
} from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import { ButtonTextWithAddIcon } from 'shared/components/ButtonTextWithAddIcon';
import { State as AttemptState } from 'shared/hooks/useAttemptNext';
import styled from 'styled-components';

import { AgentMeta } from 'teleport/Discover/useDiscover';
import { type ResourceSpec } from 'teleport/Discover/SelectResource/types';

import { SamlServiceProviderPreset } from 'teleport/services/samlidp/types';

import { H2 } from 'design';

import type { SamlGcpWorkforce } from 'teleport/services/samlidp/types';

import type { CreateSamlIdpServiceProviderRequest } from 'e-teleport/services/idp/types';

export function AttributeMapping({
  spConfig,
  setSPConfig,
  addAttrMap,
  attrMapErr,
  setAttrMapErr,
  attempt,
  agentMeta,
  resourceSpec,
}: AttrMapProps) {
  function handleInputChange(i: InputOption | InputElementChange) {
    let value;
    if (i.labelField === 'name_format' || i.labelField === 'value') {
      setAttrMapErr({ ...attrMapErr, emptyValue: false });
      value = i.option.value;
    }
    if (i.labelField === 'name') {
      setAttrMapErr({ ...attrMapErr, emptyName: false });
      value = i.event.target.value;
    }
    const newList = [...spConfig.attributeMapping];
    newList[i.index] = { ...newList[i.index], [i.labelField]: value };
    setSPConfig({ ...spConfig, attributeMapping: newList });
  }

  const addButtonTxt =
    spConfig.attributeMapping.length === 0
      ? 'Add Attribute Mapping'
      : 'Add Another Attribute Mapping';

  function removeAttribute(index: number) {
    const newList = [...spConfig.attributeMapping];
    newList.splice(index, 1);
    setAttrMapErr({ emptyName: false, emptyValue: false });
    setSPConfig({ ...spConfig, attributeMapping: newList });
  }

  function disableInput(index: number) {
    switch (resourceSpec?.samlMeta?.preset) {
      case SamlServiceProviderPreset.GcpWorkforce:
        // only one preset attribute is configured for GcpWorkforce
        // TODO(sshah): create a pre-populated GcpWorkforce preset
        // so we can get length of attributes instead of using hardcoded
        // zero index value.
        const gcpWorkforceMeta = agentMeta as SamlGcpWorkforce;
        return gcpWorkforceMeta?.isAutoConfig && index == 0;
    }
  }

  const attributeMappingDocsUrl =
    'https://goteleport.com/docs/access-controls/idps/saml-attribute-mapping/';
  function subHeading() {
    switch (resourceSpec?.samlMeta?.preset) {
      case SamlServiceProviderPreset.GcpWorkforce:
        const gcpWorkforceMeta = agentMeta as SamlGcpWorkforce;
        if (gcpWorkforceMeta?.isAutoConfig) {
          return (
            <Text typography="subtitle1" mb={4}>
              An attribute named "roles" with values containing Teleport roles
              for user will be sent by default. You can configure additional
              attribute mapping below. Please refer to the{' '}
              <Link href={attributeMappingDocsUrl} target="_blank">
                attribute mapping docs
              </Link>{' '}
              for reference.
            </Text>
          );
        }
        break;
      default:
        return (
          <Text typography="subtitle1" mb={4}>
            Teleport sends username as "uid" attribute and roles as
            "eduPersonAffiliation" attribute. If you want other attributes to
            contain username, roles or other user traits, you can specify them
            below as{' '}
            <Link href={attributeMappingDocsUrl} target="_blank">
              predicate expressions.
            </Link>
          </Text>
        );
    }
  }

  return (
    <>
      <H2 mb={2} mt={8}>
        Attribute mapping (optional)
      </H2>
      {subHeading()}
      <Box>
        {spConfig.attributeMapping.length > 0 && (
          <Flex mt={2}>
            <Box width="185px" mr={1} ml={1}>
              <Text fontSize={1}>Attribute Name</Text>
            </Box>

            <Box width="165px">
              <Text fontSize={1}>Attribute Name Format</Text>
            </Box>
            <Text fontSize={1} ml={2}>
              Attribute Value
            </Text>
          </Flex>
        )}
        {spConfig.attributeMapping.map((attribute, index) => {
          return (
            <Box mb={-5} key={index}>
              <Flex alignItems="center" mt={-3}>
                <StyledFieldInput
                  // using markAsError instead of rule so that we can udpate parent
                  // LabelInput manually.
                  markAsError={
                    attrMapErr.emptyName && attribute.name.length === 0
                  }
                  value={attribute.name}
                  placeholder="attribute_name"
                  width="190px"
                  mr={0}
                  mb={1}
                  onChange={e =>
                    handleInputChange({
                      event: e,
                      labelField: 'name',
                      index: index,
                    })
                  }
                  disabled={
                    attempt.status === 'processing' || disableInput(index)
                  }
                />
                <Box width="140px" mr={3}>
                  <StyledFieldSelect
                    mt={4}
                    isSearchable={false}
                    options={nameFormats}
                    onChange={e =>
                      handleInputChange({
                        option: e as Option,
                        labelField: 'name_format',
                        index: index,
                      })
                    }
                    defaultValue={{
                      value: urnToFriendlyName(attribute.name_format),
                      label: urnToFriendlyName(attribute.name_format),
                    }}
                    value={{
                      value: urnToFriendlyName(attribute.name_format),
                      label: urnToFriendlyName(attribute.name_format),
                    }}
                    isDisabled={
                      attempt.status === 'processing' || disableInput(index)
                    }
                  />
                </Box>
                <Box width="400px" ml={3}>
                  <FieldSelectCreatable
                    // using markAsError instead of rule so that we can udpate parent
                    // LabelInput manually.
                    markAsError={
                      attrMapErr.emptyValue && attribute.value.length === 0
                    }
                    mt={4}
                    ariaLabel="attribute value"
                    css={`
                      background: ${props => props.theme.colors.levels.surface};
                    `}
                    width="400px"
                    placeholder="write predicate expression or select from the list"
                    isSearchable
                    onChange={e => {
                      handleInputChange({
                        option: e as Option,
                        labelField: 'value',
                        index: index,
                      });
                    }}
                    // set value to be null on empty so that placeholder is shown.
                    value={
                      attribute.value.length === 0
                        ? null
                        : {
                            value: attribute.value,
                            label: attribute.value,
                          }
                    }
                    isDisabled={
                      attempt.status === 'processing' || disableInput(index)
                    }
                    createOptionPosition="last"
                    formatCreateLabel={(i: string) => 'predicate: ' + `"${i}"`}
                    options={predicateList}
                  />
                </Box>
                <ButtonIcon
                  ml={1}
                  size={1}
                  title="Remove Attribute"
                  onClick={() => removeAttribute(index)}
                  css={`
                    &:disabled {
                      opacity: 0.65;
                      pointer-events: none;
                    }
                  `}
                  disabled={
                    attempt.status === 'processing' || disableInput(index)
                  }
                >
                  <Icons.Trash size="medium" />
                </ButtonIcon>
              </Flex>
            </Box>
          );
        })}
      </Box>
      <Box mt={2} height="10px">
        <AttrMappingErrorLabel attrMapErr={attrMapErr} />
      </Box>
      <Box mt={4}>
        <ButtonTextWithAddIcon
          onClick={addAttrMap}
          label={addButtonTxt}
          disabled={attempt.status === 'processing'}
        />
      </Box>
    </>
  );
}

function AttrMappingErrorLabel({ attrMapErr: attrMapErr }) {
  return (
    <Flex mt={-4} mb={4}>
      <Box minWidth="360px" mr={1} ml={1}>
        {attrMapErr.emptyName ? (
          <LabelInput hasError={attrMapErr.emptyName}>
            Attribute name cannot be empty
          </LabelInput>
        ) : null}
      </Box>
      {attrMapErr.emptyValue ? (
        <LabelInput hasError={attrMapErr.emptyValue}>
          Attribute value cannot be empty
        </LabelInput>
      ) : null}
    </Flex>
  );
}

type attrMapErr = { emptyName: boolean; emptyValue: boolean };

const nameFormats: Option[] = [
  { value: 'unspecified', label: 'unspecified' },
  { value: 'uri', label: 'uri' },
  { value: 'basic', label: 'basic' },
];

const predicateList: Option[] = [
  { value: 'uid', label: 'uid' },
  {
    value: 'user.spec.traits.firstname',
    label: 'user.spec.traits.firstname',
  },
  {
    value: 'user.spec.traits.lastname',
    label: 'user.spec.traits.lastname',
  },
  { value: 'user.spec.roles', label: 'user.spec.roles' },
  {
    value: 'user.spec.traits.groups',
    label: 'user.spec.traits.groups',
  },
];

function urnToFriendlyName(nameFormat: string): string {
  switch (nameFormat) {
    case 'urn:oasis:names:tc:SAML:2.0:attrname-format:uri':
      return 'uri';
    case 'urn:oasis:names:tc:SAML:2.0:attrname-format:basic':
      return 'basic';
    case 'urn:oasis:names:tc:SAML:2.0:attrname-format:unspecified':
      return 'unspecified';
    default:
      return nameFormat;
  }
}

const StyledFieldInput = styled(FieldInput)`
  input {
    border-radius: 4px 0px 0px 4px;
    margin-right: -1px;
    border-right: none;
  }
`;

const StyledFieldSelect = styled(FieldSelect)`
  background: ${props => props.theme.colors.levels.surface};
  .react-select__control {
    border-radius: 0px 4px 4px 0px;
    margin-left: -1px;
  }
`;

type AttrMapProps = {
  spConfig: CreateSamlIdpServiceProviderRequest;
  setSPConfig: (SPConfig) => void;
  attrMapErr: attrMapErr;
  setAttrMapErr: (boolean) => void;
  addAttrMap: () => void;
  attempt: AttemptState['attempt'];
  resourceSpec?: ResourceSpec;
  agentMeta?: AgentMeta;
};

type InputOption = {
  labelField: 'name_format' | 'value';
  option: Option;
  index: number;
};

type InputElementChange = {
  labelField: 'name';
  event: React.ChangeEvent<HTMLInputElement>;
  index: number;
};
