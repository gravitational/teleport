import React, { ActionDispatch, useReducer, useState } from 'react';
import { components } from 'react-select';
import styled, { useTheme } from 'styled-components';
import { StyledTable as StyledTableBase } from 'web/packages/design/src/DataTable/StyledTable';

import Box from 'design/Box';
import { ButtonPrimary } from 'design/Button';
import { CardTile } from 'design/CardTile';
import Table, { Cell, LabelCell } from 'design/DataTable';
import { ClientSidePager } from 'design/DataTable/Pager';
import { PagedTableProps } from 'design/DataTable/types';
import Flex from 'design/Flex';
import * as Icons from 'design/Icon';
import { Check } from 'design/Icon';
import { H3, P1, P2 } from 'design/Text';
import { Theme } from 'design/theme/themes/types';
import { Toggle } from 'design/Toggle';
import { FieldSelectCreatable } from 'shared/components/FieldSelect';
import { CustomSelectComponentProps, Option } from 'shared/components/Select';
import Validation from 'shared/components/Validation';

import { rolesAnywhereCreateProfile } from 'teleport/Integrations/Enroll/awsLinks';

type Action =
  | { type: 'toggle'; profile: Profile }
  | { type: 'bulk'; on: boolean }
  | { type: 'filter'; filter: string };

export function ProfilesTable({
  profiles,
  refresh,
}: {
  profiles: Profile[];
  refresh: () => void;
}) {
  const [syncAll, setSyncAll] = useState(false);
  const [state, dispatch] = useReducer(reducer, []);

  function reducer(state: Profile[], action: Action): Profile[] {
    switch (action.type) {
      case 'toggle': {
        if (state.includes(action.profile)) {
          if (syncAll) {
            setSyncAll(false);
          }
          return state.filter((t: Profile) => t !== action.profile);
        } else {
          return [...state, action.profile];
        }
      }
      case 'bulk': {
        if (action.on) {
          return profiles;
        } else {
          return [];
        }
      }
      case 'filter': {
        //   todo mberg
        return state;
      }
      default: {
        throw Error('Unknown action.');
      }
    }
  }

  return (
    <Validation>
      <CardTile backgroundColor="levels.elevated">
        <Flex justifyContent="space-between">
          <Flex flexDirection="column">
            <H3>Sync IAM Profiles with Teleport as Resources</H3>
            <P2>
              You will be able to see the imported profiles on the Resources
              Page
            </P2>
          </Flex>
          <Flex alignItems="center" gap={3}>
            <ButtonPrimary
              gap={2}
              fill="minimal"
              intent="neutral"
              size="small"
              onClick={refresh}
            >
              <Icons.Refresh size="small" />
              Refresh
            </ButtonPrimary>
            <ButtonPrimary
              gap={2}
              intent="neutral"
              size="small"
              as="a"
              target="blank"
              href={rolesAnywhereCreateProfile}
            >
              Create AWS Roles Anywhere Profiles
              <Icons.NewTab size="small" />
            </ButtonPrimary>
            <Flex gap={1}>
              <Toggle
                isToggled={syncAll}
                onToggle={() => {
                  dispatch({ type: 'bulk', on: !syncAll });
                  setSyncAll(!syncAll);
                }}
                size="large"
              />
              <P1>Import All Profiles</P1>
            </Flex>
          </Flex>
        </Flex>
        {!syncAll && <Filter />}
        <Profiles
          profiles={profiles}
          loading={false}
          state={state}
          dispatch={dispatch}
        />
      </CardTile>
    </Validation>
  );
}

//  from e/web/teleport/src/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/ImportUserGroupsAndApps/CreateFilters.tsx
function Filter() {
  const theme = useTheme();
  // todo mberg hook filter into state
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
          options={[]}
          isDisabled={false}
          onChange={() => {}}
          value={[]}
          noOptionsMessage={() => null}
          label={`Filter by Profile Name(s) - Regex and glob supported`}
          rule={validFilters}
          formatCreateLabel={userInput => `Apply filter: ${userInput}`}
          stylesConfig={filterCreateCss(theme)}
          components={{
            MultiValueContainer,
            Menu: () => null,
            DropdownIndicator: () => null,
          }}
          customProps={undefined}
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

//  from e/web/teleport/src/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/ImportUserGroupsAndApps/types.ts
export type FilterOption = Option & { invalid: boolean };

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
      {lastFilter && !isLastFilter && <Text fontSize={0}>OR</Text>}
    </>
  );
};

// from e/web/teleport/src/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/ImportUserGroupsAndApps/CreateFilters.tsx
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

//   temp remove
export type Profile = {
  name: string;
  tags: string[];
  roles: string[];
};

//  from e/web/teleport/src/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/ImportUserGroupsAndApps/FilterTable.tsx
export function Profiles({
  profiles,
  loading,
  state,
  dispatch,
}: {
  profiles: Profile[];
  loading: boolean;
  state: Profile[];
  dispatch: ActionDispatch<[action: Action]>;
}) {
  function handleRowClick(row: Profile): void {
    dispatch({ type: 'toggle', profile: row });
  }

  function getRowStyle(row: Profile): React.CSSProperties {
    // todo mberg need design for non-selected row
    if (state?.includes(row)) {
      return {
        cursor: 'pointer',
        textTransform: 'uppercase',
      };
    }
    return { cursor: 'pointer', textTransform: 'lowercase' };
  }

  return (
    <Table
      data={profiles}
      row={{
        onClick: handleRowClick,
        getStyle: getRowStyle,
      }}
      columns={[
        {
          altKey: 'selected',
          headerText: '',
          render: row =>
            state?.includes(row) ? (
              <Cell width="50px">
                <Check size="small" color="success.main" />
              </Cell>
            ) : (
              <Cell width="50px" />
            ),
        },
        {
          key: 'name',
          headerText: 'Profile Name',
        },
        {
          key: 'tags',
          headerText: 'Tags',
          render: row => <LabelCell data={row.tags} />,
        },
        {
          key: 'roles',
          headerText: 'IAM Roles',
        },
      ]}
      emptyText="No Profiles Found"
      pagination={{
        pageSize: 15,
        CustomTable,
      }}
      fetching={{
        fetchStatus: loading ? 'loading' : '',
      }}
    />
  );
}

//  from e/web/teleport/src/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/ImportUserGroupsAndApps/FilterTable.tsx
function CustomTable<T>({
  nextPage,
  prevPage,
  data,
  pagination,
  renderHeaders,
  renderBody,
}: PagedTableProps<T>) {
  const { paginatedData, currentPage } = pagination;

  return (
    <>
      <TableWrapper>
        <StyledTable>
          {renderHeaders()}
          {renderBody(paginatedData[currentPage])}
        </StyledTable>
      </TableWrapper>
      <Flex justifyContent="space-between" alignItems="center">
        <Flex gap={1}>
          <Icons.Info color="text.muted" />
          <P2 color="text.muted">
            New and matching AWS Roles Anywhere Profiles created in the AWS
            Console will be automatically synced with Teleport.
          </P2>
        </Flex>
        <Flex>
          <ClientSidePager
            nextPage={nextPage}
            prevPage={prevPage}
            data={data}
            {...pagination}
          />
        </Flex>
      </Flex>
    </>
  );
}

//  from e/web/teleport/src/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/ImportUserGroupsAndApps/FilterTable.tsx

const TableWrapper = styled.div(
  props => `
      border-bottom: 1px solid ${props.theme.colors.interactive.tonal.neutral[0]};
      overflow-y: auto;
      max-height: 330px;
`
);

//  from e/web/teleport/src/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/ImportUserGroupsAndApps/FilterTable.tsx
const StyledTable = styled(StyledTableBase)(
  props => `

   background-color: inherit;
   border-collapse: separate;

  tbody > tr > td, thead > tr > th {
    font-size: ${props.theme.fontSizes[2]}px;
    font-weight: 300;
  }

  thead > tr > th {
    font-weight: bold;
    border-bottom: 1px solid ${props.theme.colors.interactive.tonal.neutral[0]};
    text-transform: none;
    padding: ${props.theme.space[2]}px 0;
    top: 0;
    position: sticky;
    z-index: 1;
    opacity: 1;
  }

  tbody > tr > td {
    padding: ${props.theme.space[3]}px 0;
  }

  tbody > tr {
    background-color: ${props.theme.colors.levels.surface};
    border: none;
    
    transition: all 150ms;
    position: relative;

    &:hover {
      border-top: ${props => props.theme.borders[2]} rgba(0, 0, 0, 0);
      background-color: ${props.theme.colors.levels.sunken};

      // We use a pseudo element for the shadow with position: absolute in order to prevent
      // the shadow from increasing the size of the layout and causing scrollbar flicker.
      &:after {
        box-shadow: ${props => props.theme.boxShadow[3]};
        content: '';
        position: absolute;
        top: 0;
        left: 0;
        z-index: -1;
        width: 100%;
        height: 100%;
      }

      + tr {
        // on hover, hide border on adjacent sibling
        border-top: ${props => props.theme.borders[2]} rgba(0, 0, 0, 0);
      }
  }
`
);
