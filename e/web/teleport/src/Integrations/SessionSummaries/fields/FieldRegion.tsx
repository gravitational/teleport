import type { OptionProps, SingleValueProps } from 'react-select';
import styled from 'styled-components';

import { Stack } from 'design/Flex';
import Text from 'design/Text';

import { FieldSelect } from 'e-teleport/Integrations/SessionSummaries/fields/FieldSelect';

interface GroupedOption {
  readonly label: string;
  readonly options: readonly Option[];
}

interface Option {
  readonly label: string;
  readonly value: string;
  readonly flag: string;
  readonly name: string;
}

interface RegionSelectorProps {
  isDisabled?: boolean;
}

export function FieldRegion({ isDisabled }: RegionSelectorProps) {
  return (
    <FieldSelect
      components={{
        Option: RegionOption,
        SingleValue: RegionSingleValue,
      }}
      isDisabled={isDisabled}
      label="Model Region"
      name="region"
      options={options}
      required={true}
    />
  );
}

const RegionOptionContainer = styled.div<{
  focused: boolean;
  selected: boolean;
}>`
  display: flex;
  padding: ${p => p.theme.space[1]}px ${p => p.theme.space[3]}px;
  background-color: ${p =>
    p.selected
      ? p.theme.colors.brand
      : p.focused
        ? p.theme.colors.interactive.tonal.neutral[0]
        : 'inherit'};
  color: ${p => (p.selected ? p.theme.colors.text.main : 'inherit')};
  cursor: pointer;
  gap: ${p => p.theme.space[3]}px;

  &:hover {
    background-color: ${p =>
      p.selected
        ? p.theme.colors.brand
        : p.theme.colors.interactive.tonal.neutral[0]};
  }
`;

function RegionOption(props: OptionProps<Option>) {
  return (
    <RegionOptionContainer
      selected={props.isSelected}
      focused={props.isFocused}
      {...props.innerProps}
    >
      <Text fontSize="large" pt={1}>
        {props.data.flag}
      </Text>

      <Stack gap={0} fontWeight={props.isSelected ? 'bold' : 'normal'}>
        <Text>{props.data.label}</Text>

        <Text
          color={props.isSelected ? 'text.main' : 'text.muted'}
          fontSize="12px"
        >
          {props.data.name}
        </Text>
      </Stack>
    </RegionOptionContainer>
  );
}

const RegionSingleValueContainer = styled.div`
  display: flex;
  align-items: center;
  gap: ${p => p.theme.space[2]}px;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  grid-area: 1 / 1 / 2 / 3;
  margin-inline-end: 0.125rem;
  margin-inline-start: 0.125rem;
`;

function RegionSingleValue(props: SingleValueProps<Option>) {
  return (
    <RegionSingleValueContainer {...props.innerProps}>
      <Text fontSize="large">{props.data.flag}</Text>

      <Text>{props.data.label}</Text>
    </RegionSingleValueContainer>
  );
}

const options: readonly GroupedOption[] = [
  {
    label: 'North America',
    options: [
      {
        label: 'us-east-1',
        value: 'us-east-1',
        flag: '🇺🇸',
        name: 'US East (N. Virginia)',
      },
      {
        label: 'us-east-2',
        value: 'us-east-2',
        flag: '🇺🇸',
        name: 'US East (Ohio)',
      },
      {
        label: 'us-west-1',
        value: 'us-west-1',
        flag: '🇺🇸',
        name: 'US West (N. California)',
      },
      {
        label: 'us-west-2',
        value: 'us-west-2',
        flag: '🇺🇸',
        name: 'US West (Oregon)',
      },
    ],
  },
  {
    label: 'Europe',
    options: [
      {
        label: 'eu-central-1',
        value: 'eu-central-1',
        flag: '🇩🇪',
        name: 'Europe (Frankfurt)',
      },
      {
        label: 'eu-central-2',
        value: 'eu-central-2',
        flag: '🇨🇭',
        name: 'Europe (Zurich)',
      },
      {
        label: 'eu-west-1',
        value: 'eu-west-1',
        flag: '🇮🇪',
        name: 'Europe (Ireland)',
      },
      {
        label: 'eu-west-2',
        value: 'eu-west-2',
        flag: '🇬🇧',
        name: 'Europe (London)',
      },
      {
        label: 'eu-west-3',
        value: 'eu-west-3',
        flag: '🇫🇷',
        name: 'Europe (Paris)',
      },
      {
        label: 'eu-south-1',
        value: 'eu-south-1',
        flag: '🇮🇹',
        name: 'Europe (Milan)',
      },
      {
        label: 'eu-south-2',
        value: 'eu-south-2',
        flag: '🇪🇸',
        name: 'Europe (Spain)',
      },
      {
        label: 'eu-north-1',
        value: 'eu-north-1',
        flag: '🇸🇪',
        name: 'Europe (Stockholm)',
      },
    ],
  },
  {
    label: 'Asia Pacific',
    options: [
      {
        label: 'ap-east-1',
        value: 'ap-east-1',
        flag: '🇭🇰',
        name: 'Asia Pacific (Hong Kong)',
      },
      {
        label: 'ap-south-1',
        value: 'ap-south-1',
        flag: '🇮🇳',
        name: 'Asia Pacific (Mumbai)',
      },
      {
        label: 'ap-south-2',
        value: 'ap-south-2',
        flag: '🇮🇳',
        name: 'Asia Pacific (Hyderabad)',
      },
      {
        label: 'ap-northeast-1',
        value: 'ap-northeast-1',
        flag: '🇯🇵',
        name: 'Asia Pacific (Tokyo)',
      },
      {
        label: 'ap-northeast-2',
        value: 'ap-northeast-2',
        flag: '🇰🇷',
        name: 'Asia Pacific (Seoul)',
      },
      {
        label: 'ap-northeast-3',
        value: 'ap-northeast-3',
        flag: '🇯🇵',
        name: 'Asia Pacific (Osaka)',
      },
      {
        label: 'ap-southeast-1',
        value: 'ap-southeast-1',
        flag: '🇸🇬',
        name: 'Asia Pacific (Singapore)',
      },
      {
        label: 'ap-southeast-2',
        value: 'ap-southeast-2',
        flag: '🇦🇺',
        name: 'Asia Pacific (Sydney)',
      },
      {
        label: 'ap-southeast-3',
        value: 'ap-southeast-3',
        flag: '🇮🇩',
        name: 'Asia Pacific (Jakarta)',
      },
      {
        label: 'ap-southeast-4',
        value: 'ap-southeast-4',
        flag: '🇦🇺',
        name: 'Asia Pacific (Melbourne)',
      },
    ],
  },
  {
    label: 'Middle East & Africa',
    options: [
      {
        label: 'af-south-1',
        value: 'af-south-1',
        flag: '🇿🇦',
        name: 'Africa (Cape Town)',
      },
      {
        label: 'il-central-1',
        value: 'il-central-1',
        flag: '🇮🇱',
        name: 'Israel (Tel Aviv)',
      },
      {
        label: 'me-south-1',
        value: 'me-south-1',
        flag: '🇧🇭',
        name: 'Middle East (Bahrain)',
      },
      {
        label: 'me-central-1',
        value: 'me-central-1',
        flag: '🇦🇪',
        name: 'Middle East (UAE)',
      },
    ],
  },
  {
    label: 'South America',
    options: [
      {
        label: 'sa-east-1',
        value: 'sa-east-1',
        flag: '🇧🇷',
        name: 'South America (São Paulo)',
      },
    ],
  },
  {
    label: 'Canada',
    options: [
      {
        label: 'ca-central-1',
        value: 'ca-central-1',
        flag: '🇨🇦',
        name: 'Canada (Central)',
      },
      {
        label: 'ca-west-1',
        value: 'ca-west-1',
        flag: '🇨🇦',
        name: 'Canada West (Calgary)',
      },
    ],
  },
];
