import { components } from 'react-select';
import { useTheme } from 'styled-components';

import { Box, Flex, Text } from 'design';
import { Theme } from 'design/theme/themes/types';
import { FieldSelectCreatable } from 'shared/components/FieldSelect';
import { CustomSelectComponentProps, Option } from 'shared/components/Select';
import { Validator } from 'shared/components/Validation';

/**
 * FilterOption extends [Select] option with a
 * boolean field to represent an invalid state.
 */
export type FilterOption = Option & { invalid: boolean };

/**
 * CreateFilters is a modified [FieldSelectCreatable] component
 * that can be used to configure resource filters for plugins.
 */
export const CreateFilters = ({
  filters,
  validator,
  label,
  onFilterChange,
  isDisabled,
  placeholder = 'Type a filter and press enter - defaults to all if no filters are defined',
  autoFocus = true,
}: {
  filters: FilterOption[];
  validator: Validator;
  label: string;
  onFilterChange(o: FilterOption[], v: Validator);
  isDisabled?: boolean;
  placeholder?: string;
  autoFocus?: boolean;
}) => {
  const theme = useTheme();
  return (
    <Flex alignItems="center" gap={2}>
      <Box width="540px">
        <FieldSelectCreatable
          ariaLabel={`input-${label}`}
          autoFocus={autoFocus}
          placeholder={placeholder}
          isMulti
          isClearable
          isSearchable
          options={filters}
          isDisabled={isDisabled}
          onChange={(o: FilterOption[]) => onFilterChange(o, validator)}
          value={filters || []}
          noOptionsMessage={() => null}
          label={label}
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

const MultiValueContainer = (
  props: CustomSelectComponentProps<{ lastFilter: string }, Option>
) => {
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

const validFilters = (createdFilters: FilterOption[]) => () => {
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
