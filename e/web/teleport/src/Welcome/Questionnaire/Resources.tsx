import { Flex, LabelInput, Text } from 'design';
import React from 'react';
import Image from 'design/Image';
import { CheckboxInput } from 'design/Checkbox';
import { useRule } from 'shared/components/Validation';
import { Option } from 'shared/components/Select';

import {
  GetResourceIcon,
  requiredResourceField,
  ResourceOptions,
} from './constants';

import { ResourceOption, ResourcesProps } from './types';
import { ResourceWrapper } from './ResourceWrapper';

export const Resources = ({ checked, updateFields }: ResourcesProps) => {
  const { valid, message } = useRule(requiredResourceField(checked));

  const updateResources = (r: Option<string, ResourceOption>) => {
    const selected = r.value as ResourceOption;
    let updated = checked;
    if (updated.includes(selected)) {
      updated = updated.filter(r => r !== (selected as ResourceOption));
    } else {
      updated.push(selected);
    }

    updateFields({ resources: updated });
  };

  const renderCheck = (resource: Option<string, ResourceOption>) => {
    const isSelected = checked.includes(resource.value as ResourceOption);
    return (
      <label
        data-testid={`box-${resource.value}`}
        key={resource.value}
        style={{
          width: '100%',
          height: '100%',
        }}
      >
        <ResourceWrapper isSelected={isSelected} invalid={!valid}>
          <CheckboxInput
            size="small"
            aria-labelledby="resources"
            data-testid={`check-${resource.value}`}
            name={resource.label}
            checked={checked.includes(resource.value as ResourceOption)}
            onChange={() => updateResources(resource)}
            style={{
              alignSelf: 'flex-end',
              margin: '0',
              outline: 'none',
            }}
          />
          <Flex
            flexDirection="column"
            alignItems="center"
            justifyContent="space-around"
            height="100%"
            gap={2}
          >
            <Image
              src={GetResourceIcon(resource.label)}
              height="64px"
              width="64px"
            />
            <Text textAlign="center" typography="paragraph2">
              {resource.label}
            </Text>
          </Flex>
        </ResourceWrapper>
      </label>
    );
  };

  return (
    <>
      <Flex gap={1} mb={1}>
        <LabelInput htmlFor={'resources'} hasError={!valid}>
          {valid
            ? `Which infrastructure resources do you need to access frequently?`
            : message}
          <i>&nbsp;Select all that apply.</i>
        </LabelInput>
      </Flex>
      <Flex gap={2} alignItems="flex-start" height="160px">
        {ResourceOptions.map((r: Option<string, ResourceOption>) =>
          renderCheck(r)
        )}
      </Flex>
    </>
  );
};
