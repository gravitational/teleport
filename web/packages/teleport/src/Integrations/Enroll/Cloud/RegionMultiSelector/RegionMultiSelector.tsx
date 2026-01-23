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

import { RegionGroup, RegionId } from './types';

type RegionOption = Option<RegionId>;
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

function OptionWithCheckbox(
  props: OptionProps<RegionOption, true, RegionOptionGroup>
) {
  return (
    <components.Option {...props}>
      <Flex alignItems="center" gap={2}>
        <CheckboxInput checked={props.isSelected} />
        <span>{props.children}</span>
      </Flex>
    </components.Option>
  );
}

function defaultRule(): Rule<RegionId[]> {
  return () => () => ({ valid: true });
}

export interface RegionMultiSelectorProps {
  regionGroups: readonly RegionGroup[];
  selectedRegions: RegionId[];
  onChange(regions: RegionId[]): void;
  label?: string;
  placeholder?: string;
  disabled?: boolean;
  required?: boolean;
  rule?: Rule<RegionId[]>;
}

export function RegionMultiSelector({
  regionGroups,
  selectedRegions,
  onChange,
  label = 'Select regions',
  placeholder = 'Select regions...',
  disabled = false,
  required = false,
  rule = defaultRule(),
}: RegionMultiSelectorProps) {
  const { valid, message } = useRule(rule(selectedRegions));

  const groups: RegionOptionGroup[] = regionGroups.map(regionGroup => ({
    label: regionGroup.name,
    options: regionGroup.regions.map(region => ({
      value: region.id,
      label: region.name,
    })),
  }));

  const selectedOptions: RegionOption[] = groups
    .flatMap(group => group.options)
    .filter(option => selectedRegions.includes(option.value));

  const handleChange = (options: RegionOption[]) => {
    const regions = options ? options.map(option => option.value) : [];
    onChange(regions);
  };

  const hasError = !valid;

  return (
    <Container>
      <LabelInput mb={0}>
        <LabelContent required={required} mb={1}>
          {label}
        </LabelContent>
        <StyledSelect
          isMulti
          options={groups}
          value={selectedOptions}
          onChange={handleChange}
          placeholder={placeholder}
          isDisabled={disabled}
          isSearchable={false}
          closeMenuOnSelect={false}
          hideSelectedOptions={false}
          size="large"
          hasError={hasError}
          components={{
            Option: OptionWithCheckbox,
            ValueContainer: MultiRegionValueContainer,
            MultiValue: () => null,
          }}
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
