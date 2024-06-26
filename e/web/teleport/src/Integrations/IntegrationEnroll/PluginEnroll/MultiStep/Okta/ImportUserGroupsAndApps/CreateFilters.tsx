import React from 'react';
import { components, MultiValueGenericProps } from 'react-select';
import { useTheme } from 'styled-components';
import { Box, Flex, Text } from 'design';
import { Attempt } from 'shared/hooks/useAttemptNext';
import { Theme } from 'design/theme/themes/types';

import { FieldSelectCreatable } from 'shared/components/FieldSelect';
import { Validator } from 'shared/components/Validation';

import { FormDataField } from '../types';

import { FilterOption, FormDataFilterField } from './types';

export const CreateFilters = ({
  filters,
  validator,
  filterKind,
  onFilterChange,
  importAttempt,
  filterAttempt,
}: {
  filters: FilterOption[];
  validator: Validator;
  filterKind: FormDataFilterField;
  onFilterChange(o: FilterOption[], v: Validator, k: FormDataFilterField);
  importAttempt: Attempt;
  filterAttempt: Attempt;
}) => {
  const theme = useTheme();
  return (
    <Flex alignItems="center" gap={2}>
      <Box width="540px">
        <FieldSelectCreatable
          ariaLabel={`input-${filterKind === FormDataField.AppFilters ? 'app' : 'group'}`}
          autoFocus={true}
          placeholder="Type a filter and press enter - defaults to all if no filters are defined"
          isMulti
          isClearable
          isSearchable
          options={filters}
          isDisabled={
            filterAttempt.status === 'processing' ||
            importAttempt.status === 'processing'
          }
          onChange={(o: FilterOption[]) =>
            onFilterChange(o, validator, filterKind)
          }
          value={filters || []}
          noOptionsMessage={() => null}
          label={`Filter by ${filterKind === FormDataField.AppFilters ? 'App' : 'Group'} Name(s) - Regex and glob supported`}
          rule={validFilters}
          formatCreateLabel={userInput => `Apply filter: ${userInput}`}
          stylesConfig={filterCreateCss(theme)}
          components={{
            MultiValueContainer,
            Menu: () => null,
            DropdownIndicator: () => null,
          }}
          customProps={{
            lastFilter:
              filters?.length > 1 ? filters[filters.length - 1].value : '',
          }}
        />
      </Box>
    </Flex>
  );
};

const filterCreateCss = (theme: Theme) => ({
  multiValue: (base, state) => {
    const errorState = state.data.invalid
      ? { border: `1px solid ${theme.colors.error.main}` }
      : undefined;

    return {
      ...base,
      ...errorState,
    };
  },
});

const MultiValueContainer = (props: MultiValueGenericProps) => {
  const lastFilter = props.selectProps.customProps.lastFilter;
  const currFilter = props.data.value;

  const isLastFilter = lastFilter === currFilter;
  return (
    <>
      <components.MultiValueContainer {...props} />
      {lastFilter && !isLastFilter && <Text fontSize={0}>OR</Text>}
    </>
  );
};

export const validFilters = (createdFilters: FilterOption[]) => () => {
  if (!createdFilters) {
    return {
      valid: true,
    };
  }

  const badFilters = createdFilters.filter(f => f.invalid);

  if (badFilters.length > 0) {
    return {
      valid: false,
      message: `The following filters are invalid: ${badFilters
        .map(f => f.value)
        .join(', ')}`,
    };
  }

  return { valid: true };
};
