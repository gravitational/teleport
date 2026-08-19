import React from 'react';
import styled from 'styled-components';

import {
  Box,
  ButtonIcon,
  Flex,
  H2,
  LabelInput,
  Link,
  Mark,
  Text,
} from 'design';
import * as Icons from 'design/Icon';
import { P } from 'design/Text/Text';
import { ButtonWithAddIcon } from 'shared/components/ButtonWithAddIcon';
import FieldInput from 'shared/components/FieldInput';
import {
  FieldSelect,
  FieldSelectCreatable,
} from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';

import {
  gcpWorkforcePresetSpec,
  microsoftEntraIdPresetSpec,
  type CreateSamlIdpServiceProviderRequest,
} from 'e-teleport/services/idp/types';
import {
  SamlServiceProviderPreset,
  type AttributeMapping as AttributeMappingType,
} from 'teleport/services/samlidp/types';

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

  return (
    <>
      <H2 mb={2} mt={8}>
        Attribute mapping (optional)
      </H2>
      <SubHeading preset={preset} isGuided={isGuided} />
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
            const disablePresetEdit = disableAttributeRow(
              preset,
              isGuided,
              attribute
            );
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
                    disabled={disabled || disablePresetEdit}
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
                      isDisabled={disabled || disablePresetEdit}
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
                      isDisabled={disabled || disablePresetEdit}
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
                    disabled={disabled || disablePresetEdit}
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
        <ButtonWithAddIcon
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

function SubHeading({
  preset,
  isGuided,
}: {
  preset: SamlServiceProviderPreset;
  isGuided: boolean;
}) {
  const docsUrl =
    'https://goteleport.com/docs/admin-guides/access-controls/idps/saml-attribute-mapping/';
  const DocsCopy = (
    <>
      Please refer to the{' '}
      <Link href={docsUrl} target="_blank">
        attribute mapping docs
      </Link>{' '}
      for reference.
    </>
  );
  const Default = (
    <P mb={4}>
      Teleport sends username as "uid" attribute and roles as
      "eduPersonAffiliation" attribute. If you want other attributes to contain
      username, roles or other user traits, you can specify them as predicate
      expressions. {DocsCopy}
    </P>
  );
  switch (preset) {
    case SamlServiceProviderPreset.GcpWorkforce:
      if (isGuided) {
        return (
          <P mb={4}>
            An attribute named "roles" with values containing Teleport roles for
            user will be sent by default. You can configure additional attribute
            mapping below. {DocsCopy}
          </P>
        );
      }
      return Default;
    case SamlServiceProviderPreset.MicrosoftEntraId:
      return (
        <P mb={4}>
          An attribute named &nbsp;
          <Mark>
            http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress
          </Mark>
          &nbsp; with value containing Teleport username will be sent by
          default. You can configure additional attribute mapping below.{' '}
          {DocsCopy}
        </P>
      );
    default:
      return Default;
  }
}

/**
 * Checks if attribute row should be disabled.
 * Only the attribute name value is expected to be unique.
 */
function disableAttributeRow(
  preset: SamlServiceProviderPreset,
  isGuided: boolean,
  currentAttribute: AttributeMappingType
): boolean {
  if (!isGuided) {
    return false;
  }

  switch (preset) {
    case SamlServiceProviderPreset.GcpWorkforce:
      return gcpWorkforcePresetSpec().attribute_mapping.some(
        presetAttribute => presetAttribute.name === currentAttribute.name
      );
    case SamlServiceProviderPreset.MicrosoftEntraId:
      return microsoftEntraIdPresetSpec().attribute_mapping.some(
        presetAttribute => presetAttribute.name === currentAttribute.name
      );
    default:
      return false;
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
