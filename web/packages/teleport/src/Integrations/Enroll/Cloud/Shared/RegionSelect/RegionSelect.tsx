/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { components, OptionProps, ValueContainerProps } from 'react-select';
import styled from 'styled-components';

import { Box, Flex, LabelInput, Text } from 'design';
import { CheckboxInput } from 'design/Checkbox';
import { LabelContent } from 'design/LabelInput/LabelInput';
import { HelperTextLine } from 'shared/components/FieldInput/FieldInput';
import Select, { Option } from 'shared/components/Select';
import { useRule } from 'shared/components/Validation';
import { Rule } from 'shared/components/Validation/rules';

import { RegionGroup } from './types';

type RegionOption = Option<string, React.ReactNode>;
type RegionOptionGroup = {
  label: string;
  options: RegionOption[];
};

function MultiRegionValueContainer(
  props: ValueContainerProps<RegionOption, true, RegionOptionGroup>
) {
  const { children, selectProps } = props;
  const selectedCount = Array.isArray(selectProps.value)
    ? selectProps.value.length
    : 0;

  return (
    <components.ValueContainer {...props}>
      {selectedCount > 0 && (
        <Text fontSize={2} fontWeight="light" color="muted">
          {`${selectedCount} region${selectedCount !== 1 ? 's' : ''} selected`}
        </Text>
      )}
      {children}
    </components.ValueContainer>
  );
}

function SingleRegionValue(props: any) {
  const option = props.data as RegionOption;
  return (
    <components.SingleValue {...props}>
      <Text fontSize={2} fontWeight="light" color="muted">
        {option.value}
      </Text>
    </components.SingleValue>
  );
}

function OptionWithCheckbox(
  props: OptionProps<RegionOption, true, RegionOptionGroup>
) {
  return (
    <components.Option {...props}>
      <Flex gap={2}>
        <CheckboxInput checked={props.isSelected} />
        <Flex flex="1">{props.children}</Flex>
      </Flex>
    </components.Option>
  );
}

function defaultRule(): Rule<string[] | string> {
  return () => () => ({ valid: true });
}

interface BaseRegionSelectorProps {
  regionGroups: readonly RegionGroup[];
  label?: string;
  placeholder?: string;
  disabled?: boolean;
  required?: boolean;
}

interface RegionSelectPropsMulti extends BaseRegionSelectorProps {
  isMulti?: true;
  selectedRegions: string[];
  onChange(regions: string[]): void;
  rule?: Rule<string[]>;
}

interface RegionSelectPropsSingle extends BaseRegionSelectorProps {
  isMulti: false;
  selectedRegions: string;
  onChange(region: string): void;
  rule?: Rule<string>;
}

export type RegionSelectProps =
  | RegionSelectPropsMulti
  | RegionSelectPropsSingle;

export function RegionSelect(props: RegionSelectProps) {
  const {
    regionGroups,
    selectedRegions,
    onChange,
    label = 'Select regions',
    placeholder = 'Select regions...',
    disabled = false,
    required = false,
    rule = defaultRule(),
    isMulti = true,
  } = props;

  const { valid, message } = useRule(rule(selectedRegions));

  const groups: RegionOptionGroup[] = regionGroups.map(regionGroup => ({
    label: regionGroup.name,
    options: regionGroup.regions.map(region => ({
      value: region.id,
      label: (
        <Flex justifyContent="space-between" width="100%">
          <Text as="span">{region.name}</Text>
          <Text as="span">{region.id}</Text>
        </Flex>
      ),
    })),
  }));

  const allOptions = groups.flatMap(group => group.options);

  const selectedOptions = isMulti
    ? allOptions.filter(option =>
        (selectedRegions as string[]).includes(option.value)
      )
    : allOptions.find(option => option.value === selectedRegions) || null;

  const handleChange = (options: RegionOption[] | RegionOption) => {
    if (isMulti) {
      const regions = options
        ? (options as RegionOption[]).map(option => option.value)
        : [];
      (onChange as (regions: string[]) => void)(regions);
    } else {
      const region = (options as RegionOption)?.value || '';
      (onChange as (region: string) => void)(region);
    }
  };

  const hasError = !valid;

  const selectComponents = isMulti
    ? {
        Option: OptionWithCheckbox,
        ValueContainer: MultiRegionValueContainer,
        MultiValue: () => null,
      }
    : {
        SingleValue: SingleRegionValue,
      };

  return (
    <Container>
      <LabelInput mb={0}>
        <LabelContent required={required} mb={1}>
          {label}
        </LabelContent>
        <StyledSelect
          isMulti={isMulti}
          options={groups}
          value={selectedOptions}
          onChange={handleChange}
          placeholder={placeholder}
          isDisabled={disabled}
          isSearchable={false}
          closeMenuOnSelect={!isMulti}
          hideSelectedOptions={false}
          size="large"
          hasError={hasError}
          components={selectComponents}
        />
      </LabelInput>
      <HelperTextLine
        hasError={hasError}
        helperTextId="regions-error"
        errorMessage={message}
      />
    </Container>
  );
}

const Container = styled(Box)`
  width: 400px;
`;

const StyledSelect = styled(Select)`
  .react-select__group-heading {
    font-size: ${p => p.theme.fontSizes[1]}px;
    color: ${p => p.theme.colors.text.slightlyMuted};
  }

  .react-select__placeholder {
    font-size: ${p => p.theme.fontSizes[2]}px;
    color: ${p => p.theme.colors.text.slightlyMuted};
  }
`;
