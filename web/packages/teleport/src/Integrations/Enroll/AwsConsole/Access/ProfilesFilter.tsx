/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { Dispatch, SetStateAction } from 'react';
import { components } from 'react-select';
import { useTheme } from 'styled-components';

import Box from 'design/Box';
import Flex from 'design/Flex';
import { P1 } from 'design/Text';
import { Theme } from 'design/theme/themes/types';
import { FieldSelectCreatable } from 'shared/components/FieldSelect';
import { CustomSelectComponentProps, Option } from 'shared/components/Select';

export type ProfilesFilterOption = Option & { invalid: boolean };

export function ProfilesFilter({
  filters,
  setFilters,
}: {
  filters: ProfilesFilterOption[];
  setFilters: Dispatch<SetStateAction<ProfilesFilterOption[]>>;
}) {
  const theme = useTheme();

  return (
    <Flex alignItems="center" gap={2}>
      <Box width="540px">
        <FieldSelectCreatable
          ariaLabel={`input-profiles`}
          autoFocus={true}
          placeholder="* – Type a filter to restrict matches"
          isMulti
          isClearable
          isSearchable
          options={filters}
          onChange={(o: ProfilesFilterOption[]) => setFilters(o)}
          value={filters || []}
          noOptionsMessage={() => null}
          label="Filter by Profile Name"
          helperText="Regex and glob supported. Defaults to all(*) if no filters are defined"
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
}

const filterCreateCss = (theme: Theme) => ({
  multiValue: (base, state) => {
    const errorState = state.data.invalid
      ? { border: `1px solid ${theme.colors.interactive.solid.danger.default}` }
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
      {lastFilter && !isLastFilter && <P1 fontSize={0}>OR</P1>}
    </>
  );
};
