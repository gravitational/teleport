import React from 'react';
import styled from 'styled-components';

import { Box, ButtonIcon, Flex, H2, LabelInput, Link, Text } from 'design';
import * as Icons from 'design/Icon';
import { P } from 'design/Text/Text';
import { ButtonTextWithAddIcon } from 'shared/components/ButtonTextWithAddIcon';
import FieldInput from 'shared/components/FieldInput';
import {
  FieldSelect,
  FieldSelectCreatable,
} from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';

import type { CreateSamlIdpServiceProviderRequest } from 'e-teleport/services/idp/types';
import { SamlServiceProviderPreset } from 'teleport/services/samlidp/types';

export function AttributeMapping({
  spConfig,
  setSPConfig,
  addAttrMap,
  attrMapErr,
  setAttrMapErr,
  disabled,
  preset,
  isGuided,
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

  function disableAttributeRow(index: number) {
    switch (preset) {
      case SamlServiceProviderPreset.GcpWorkforce:
        // only one preset attribute is configured for GcpWorkforce
        // TODO(sshah): create a pre-populated GcpWorkforce preset
        // so we can get length of attributes instead of using hardcoded
        // zero index value.
        return isGuided && index == 0;
    }
  }

  const attributeMappingDocsUrl =
    'https://goteleport.com/docs/access-controls/idps/saml-attribute-mapping/';

  function SubHeading() {
    switch (preset) {
      case SamlServiceProviderPreset.GcpWorkforce:
        if (isGuided) {
          return (
            <P mb={4}>
              An attribute named "roles" with values containing Teleport roles
              for user will be sent by default. You can configure additional
              attribute mapping below. Please refer to the{' '}
              <Link href={attributeMappingDocsUrl} target="_blank">
                attribute mapping docs
              </Link>{' '}
              for reference.
            </P>
          );
        }
        break;
      default:
        return (
          <P mb={4}>
            Teleport sends username as "uid" attribute and roles as
            "eduPersonAffiliation" attribute. If you want other attributes to
            contain username, roles or other user traits, you can specify them
            below as{' '}
            <Link href={attributeMappingDocsUrl} target="_blank">
              predicate expressions.
            </Link>
          </P>
        );
    }
  }

  return (
    <>
      <H2 mb={2} mt={8}>
        Attribute mapping (optional)
      </H2>
      <SubHeading />
      <Box>
        {spConfig.attributeMapping.length > 0 && (
          <Flex mt={2} mb={1}>
            <Box width="185px" mr={1} ml={1}>
              <Text typography="body3">Attribute Name</Text>
            </Box>

            <Box width="165px">
              <Text typography="body3">Attribute Name Format</Text>
            </Box>
            <Text typography="body3" ml={2}>
              Attribute Value
            </Text>
          </Flex>
        )}
        <Flex flexDirection="column" gap={3}>
          {spConfig.attributeMapping.map((attribute, index) => {
            return (
              <Box key={index}>
                <Flex alignItems="center">
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
                    mb={0}
                    onChange={e =>
                      handleInputChange({
                        event: e,
                        labelField: 'name',
                        index: index,
                      })
                    }
                    disabled={disabled || disableAttributeRow(index)}
                  />
                  <Box width="140px" mr={3}>
                    <StyledFieldSelect
                      mb={0}
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
                      isDisabled={disabled || disableAttributeRow(index)}
                    />
                  </Box>
                  <Box width="400px" ml={3}>
                    <FieldSelectCreatable
                      mb={0}
                      // using markAsError instead of rule so that we can udpate parent
                      // LabelInput manually.
                      markAsError={
                        attrMapErr.emptyValue && attribute.value.length === 0
                      }
                      ariaLabel="attribute value"
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
                      isDisabled={disabled || disableAttributeRow(index)}
                      createOptionPosition="last"
                      formatCreateLabel={(i: string) =>
                        'predicate: ' + `"${i}"`
                      }
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
                    disabled={disabled || disableAttributeRow(index)}
                  >
                    <Icons.Trash size="medium" />
                  </ButtonIcon>
                </Flex>
              </Box>
            );
          })}
        </Flex>
      </Box>
      <Box mt={1} height="10px">
        <AttrMappingErrorLabel attrMapErr={attrMapErr} />
      </Box>
      <Box mt={4}>
        <ButtonTextWithAddIcon
          onClick={addAttrMap}
          label={addButtonTxt}
          disabled={disabled}
        />
      </Box>
    </>
  );
}

function AttrMappingErrorLabel({ attrMapErr: attrMapErr }) {
  return (
    <Flex mb={4}>
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

export type attrMapErr = { emptyName: boolean; emptyValue: boolean };

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
  disabled: boolean;
  preset: SamlServiceProviderPreset;
  isGuided: boolean;
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
