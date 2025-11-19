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

import { components, ValueContainerProps } from 'react-select';
import styled from 'styled-components';

import { Box, Flex, Text } from 'design';
import Select, { Option } from 'shared/components/Select';

import { awsRegionMap, Regions } from 'teleport/services/integrations';

type RegionOption = Option<Regions>;
type RegionGroup = {
  label: string;
  options: RegionOption[];
};

const createRegionGroups = (): RegionGroup[] => {
  const availabilityZones = [
    {
      label: 'North America',
      regions: [
        'ca-central-1',
        'us-east-1',
        'us-east-2',
        'us-west-1',
        'us-west-2',
      ] as Regions[],
    },
    {
      label: 'South America',
      regions: ['sa-east-1'] as Regions[],
    },
    {
      label: 'Asia Pacific',
      regions: [
        'ap-east-1',
        'ap-south-2',
        'ap-south-1',
        'ap-northeast-3',
        'ap-northeast-2',
        'ap-southeast-1',
        'ap-southeast-2',
        'ap-southeast-4',
        'ap-southeast-3',
        'ap-northeast-1',
      ] as Regions[],
    },
    {
      label: 'Europe',
      regions: [
        'eu-central-1',
        'eu-central-2',
        'eu-west-1',
        'eu-west-2',
        'eu-west-3',
        'eu-south-1',
        'eu-south-2',
        'eu-north-1',
      ] as Regions[],
    },
    {
      label: 'Middle East',
      regions: ['me-south-1', 'me-central-1', 'il-central-1'] as Regions[],
    },
    {
      label: 'Africa',
      regions: ['af-south-1'] as Regions[],
    },
  ];

  return availabilityZones.map(zone => ({
    label: zone.label,
    options: zone.regions
      .filter(region => region in awsRegionMap)
      .map(region => ({
        value: region,
        label: `${awsRegionMap[region]} (${region})`,
      })),
  }));
};

const MultiValue = () => {
  return null;
};

const Placeholder = () => {
  return null;
};

const ValueContainer = (props: ValueContainerProps<RegionOption[], true>) => {
  const { children, selectProps } = props;
  const selectedCount = selectProps.value?.length || 0;

  const displayText =
    selectedCount > 0
      ? `${selectedCount} region${selectedCount !== 1 ? 's' : ''} selected`
      : selectProps.placeholder || 'Select regions...';

  return (
    <components.ValueContainer {...props}>
      <Flex
        alignItems="center"
        height="100%"
        pl="12px"
        color={selectedCount > 0 ? 'text.main' : 'text.muted'}
        css={{
          pointerEvents: 'none',
          position: 'absolute',
          top: 0,
          left: 0,
          right: 0,
        }}
      >
        {displayText}
      </Flex>
      {children}
    </components.ValueContainer>
  );
};

export function RegionMultiSelector({
  selectedRegions,
  onChange,
  disabled = false,
}: {
  selectedRegions: Regions[];
  onChange(regions: Regions[]): void;
  disabled?: boolean;
}) {
  const regionGroups = createRegionGroups();

  const selectedOptions: RegionOption[] = selectedRegions.map(region => ({
    value: region,
    label: `${awsRegionMap[region]} (${region})`,
  }));

  const handleChange = (options: RegionOption[]) => {
    const regions = options ? options.map(option => option.value) : [];
    onChange(regions);
  };

  return (
    <Container>
      <LabelContainer>
        <Text color="text.main">Select regions</Text>
        <RequiredIndicator>*</RequiredIndicator>
      </LabelContainer>

      <StyledSelect
        isMulti={true}
        options={regionGroups}
        value={selectedOptions}
        onChange={handleChange}
        placeholder="Select regions..."
        isDisabled={disabled}
        isSearchable={false}
        closeMenuOnSelect={false}
        hideSelectedOptions={false}
        size="large"
        components={{
          MultiValue,
          ValueContainer,
          Placeholder,
        }}
      />
    </Container>
  );
}

// Styled Components
const Container = styled(Box)`
  width: 400px;
`;

const LabelContainer = styled(Flex)`
  align-items: center;
  margin-bottom: 8px;
`;

const RequiredIndicator = styled(Text)`
  color: ${p => p.theme.colors.error.main};
  margin-left: 4px;
`;

const StyledSelect = styled(Select)`
  .react-select__menu {
    max-height: 300px;
  }

  .react-select__group-heading {
    font-weight: 600;
    color: ${p => p.theme.colors.text.slightlyMuted};
    background: ${p => p.theme.colors.levels.surface};
    padding: 8px 12px;
    font-size: 13px;
    text-transform: none;
    letter-spacing: normal;
    margin-bottom: 0;
  }

  .react-select__option {
    padding: 8px 12px;
  }
`;
