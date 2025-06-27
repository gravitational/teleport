import { Dispatch, SetStateAction } from 'react';
import { components } from 'react-select';
import { useTheme } from 'styled-components';

import Box from 'design/Box';
import Flex from 'design/Flex';
import { P1 } from 'design/Text';
import { Theme } from 'design/theme/themes/types';
import { FieldSelectCreatable } from 'shared/components/FieldSelect';
import { CustomSelectComponentProps, Option } from 'shared/components/Select';

//  from e/web/teleport/src/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/ImportUserGroupsAndApps/types.ts
export type ProfilesFilterOption = Option & { invalid: boolean };

//  from e/web/teleport/src/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/ImportUserGroupsAndApps/CreateFilters.tsx
export function ProfilesFilter({
  filters,
  setFilters,
}: {
  filters: ProfilesFilterOption[];
  setFilters: Dispatch<SetStateAction<ProfilesFilterOption[]>>;
}) {
  const theme = useTheme();
  // todo mberg 2 hook filter into state

  // from e/web/teleport/src/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/ImportUserGroupsAndApps/CreateFilters.tsx
  const validFilters = (createdFilters: ProfilesFilterOption[]) => () => {
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

  return (
    <Flex alignItems="center" gap={2}>
      <Box width="540px">
        <FieldSelectCreatable
          ariaLabel={`input-profiles`}
          autoFocus={true}
          placeholder="Type a filter and press enter - defaults to all if no filters are defined"
          isMulti
          isClearable
          isSearchable
          options={filters}
          onChange={() => {
            //   todo
          }}
          value={filters || []}
          // noOptionsMessage={() => null}
          label={`Filter by Profile Name(s) - Regex and glob supported`}
          rule={validFilters}
          formatCreateLabel={userInput => `Apply filter: ${userInput}`}
          stylesConfig={filterCreateCss(theme)}
          components={{
            MultiValueContainer,
            Menu: () => null,
            DropdownIndicator: () => null,
          }}
          customProps={undefined} //todo mberg
        />
      </Box>
    </Flex>
  );
}

//  from e/web/teleport/src/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/ImportUserGroupsAndApps/CreateFilters.tsx
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

// from e/web/teleport/src/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/ImportUserGroupsAndApps/CreateFilters.tsx
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
