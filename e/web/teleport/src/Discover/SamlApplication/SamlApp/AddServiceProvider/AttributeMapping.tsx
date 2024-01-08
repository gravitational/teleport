import React from 'react';
import { Box, ButtonIcon, Flex, Text, LabelInput } from 'design';
import * as Icons from 'design/Icon';
import FieldInput from 'shared/components/FieldInput';
import FieldSelect, {
  FieldSelectCreatable,
} from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import { ButtonTextWithAddIcon } from 'shared/components/ButtonTextWithAddIcon';
import { State as AttemptState } from 'shared/hooks/useAttemptNext';
import styled from 'styled-components';

import type { CreateSamlIdpServiceProviderRequest } from 'e-teleport/services/idp/types';

export function AttributeMapping({
  spConfig,
  setSPConfig,
  addAttrMap,
  attrMapErr,
  setAttrMapErr,
  attempt,
}: AttrMapProps) {
  function handleInputChange(i: InputOption | InputElementChange) {
    let value;
    if (i.labelField === 'nameFormat' || i.labelField === 'value') {
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
      <Text fontSize="18px" typography="subtitle1" mb={2} mt={8}>
        Attribute mapping (optional)
      </Text>
      <Text typography="subtitle1" mb={4}>
        Teleport sends username as "uid" attribute and roles as
        "eduPersonAffiliation" attribute. If you want other attributes to
        contain username, roles or other user traits, you can specify them below
        as predicate expressions.
        <br />
      </Text>
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
                  disabled={attempt.status === 'processing'}
                />
                <Box width="140px" mr={3}>
                  <StyledFieldSelect
                    mt={4}
                    isSearchable={false}
                    options={nameFormats}
                    onChange={e =>
                      handleInputChange({
                        option: e,
                        labelField: 'nameFormat',
                        index: index,
                      })
                    }
                    defaultValue={{
                      value: attribute.nameFormat,
                      label: attribute.nameFormat,
                    }}
                    value={{
                      value: attribute.nameFormat,
                      label: attribute.nameFormat,
                    }}
                    isDisabled={attempt.status === 'processing'}
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
                    isDisabled={attempt.status === 'processing'}
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
                  disabled={attempt.status === 'processing'}
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
};

type InputOption = {
  labelField: 'nameFormat' | 'value';
  option: Option;
  index: number;
};

type InputElementChange = {
  labelField: 'name';
  event: React.ChangeEvent<HTMLInputElement>;
  index: number;
};
