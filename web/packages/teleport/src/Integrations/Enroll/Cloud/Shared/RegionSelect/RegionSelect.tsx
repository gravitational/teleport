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

import {
  components,
  GroupBase,
  OptionProps,
  ValueContainerProps,
  StylesConfig,
} from 'react-select';
import { useTheme } from 'styled-components';

import { Flex, Text } from 'design';
import { CheckboxInput } from 'design/Checkbox';
import { FieldSelect } from 'shared/components/FieldSelect';
import { FieldProps } from 'shared/components/FieldSelect/shared';
import { Option, Props as SelectProps } from 'shared/components/Select';

import { CloudRegion } from '../types';

function OptionContainer<
  R extends CloudRegion,
  IsMulti extends boolean = false,
>(props: OptionProps<Option<R>, IsMulti>) {
  return (
    <components.Option {...props}>
      <Flex gap={2} width="100%">
        {props.isMulti && <CheckboxInput checked={props.isSelected} />}
        <Flex flex="1" justifyContent="space-between">
          <Text as="span">{props.children}</Text>
          <Text as="span">{props.data.value}</Text>
        </Flex>
      </Flex>
    </components.Option>
  );
}

function MultiValueContainer<R extends CloudRegion>(
  props: ValueContainerProps<Option<R>>
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

type BaseProps<R extends CloudRegion, IsMulti extends boolean> = Omit<
  SelectProps<Option<R>, IsMulti, GroupBase<Option<R>>> &
    FieldProps<Option<R>, IsMulti>,
  'onChange'
>;

type MultiSelectProps<R extends CloudRegion> = BaseProps<R, true> & {
  isMulti: true;
  onChange: (options: readonly Option<R>[]) => void;
};

type SingleSelectProps<R extends CloudRegion> = BaseProps<R, false> & {
  isMulti: false;
  onChange: (option: Option<R> | null) => void;
};

export type RegionSelectProps<R extends CloudRegion> =
  | MultiSelectProps<R>
  | SingleSelectProps<R>;

const defaultProps = {
  isSearchable: false,
  hideSelectedOptions: false,
} as const;

function MultiSelect<R extends CloudRegion>(props: MultiSelectProps<R>) {
  return (
    <FieldSelect<Option<R>, true>
      {...props}
      {...defaultProps}
      placeholder={props.placeholder ?? 'Select regions...'}
      closeMenuOnSelect={false}
      isMulti={true}
      components={{
        Option: OptionContainer<R, true>,
        ValueContainer: MultiValueContainer<R>,
        MultiValue: () => null,
      }}
    />
  );
}

function SingleSelect<R extends CloudRegion>(props: SingleSelectProps<R>) {
  return (
    <FieldSelect<Option<R>, false>
      {...props}
      {...defaultProps}
      placeholder={props.placeholder ?? 'Select region...'}
      isMulti={false}
      components={{
        Option: OptionContainer<R, false>,
      }}
    />
  );
}

export function RegionSelect<R extends CloudRegion>(
  props: RegionSelectProps<R>
) {
  const theme = useTheme();
  const stylesConfig: StylesConfig = {
    groupHeading: base => ({
      ...base,
      fontSize: `${theme.fontSizes[1]}px`,
      color: theme.colors.text.slightlyMuted,
    }),
  };

  if (props.isMulti === true) {
    return <MultiSelect {...props} stylesConfig={stylesConfig} />;
  }
  return <SingleSelect {...props} stylesConfig={stylesConfig} />;
}
