import { useState } from 'react';
import { components } from 'react-select';
import styled, { useTheme } from 'styled-components';

import Box from 'design/Box';
import { CardTile } from 'design/CardTile';
import Table from 'design/DataTable';
import { ClientSidePager } from 'design/DataTable/Pager';
import { StyledTable as StyledTableBase } from 'design/DataTable/StyledTable';
import { PagedTableProps } from 'design/DataTable/types';
import Flex from 'design/Flex';
import { H2, H3, P1, P2 } from 'design/Text';
import { Theme } from 'design/theme/themes/types';
import { Toggle } from 'design/Toggle';
import { FieldSelectCreatable } from 'shared/components/FieldSelect';
import { CustomSelectComponentProps, Option } from 'shared/components/Select';
import Validation from 'shared/components/Validation';

export function Access() {
  const [syncAll, setSyncAll] = useState(false);

  return (
    <Box pt={3}>
      <H2>Configure Access</H2>
      <P2>
        Import and synchronize <b>AWS IAM Roles Anywhere</b> Profiles into
        Teleport. Imported Profiles will be available as Resources with each
        Role available as an account.
      </P2>
      <Validation>
        <CardTile>
          <Flex justifyContent="space-between">
            <Flex flexDirection="column">
              <H3>Sync IAM Profiles with Teleport as Resources</H3>
              <P2>
                You will be able to see the imported profiles on the Resources
                Page
              </P2>
            </Flex>
            <Flex flexDirection="row" alignItems="center" gap={3}>
              <Toggle
                isToggled={syncAll}
                onToggle={() => setSyncAll(!syncAll)}
                size="large"
              />
              <P1>Import All Profiles</P1>
            </Flex>
          </Flex>
          {!syncAll && <Filter />}
          <Profiles profiles={[]} loading={false} />
        </CardTile>
      </Validation>
    </Box>
  );
}

//  from e/web/teleport/src/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/ImportUserGroupsAndApps/CreateFilters.tsx
function Filter() {
  const theme = useTheme();
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
type profile = {
  name: string;
  tags: string[];
  roles: string[];
};

//  from e/web/teleport/src/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/ImportUserGroupsAndApps/FilterTable.tsx
export function Profiles({
  profiles,
  loading,
}: {
  profiles: profile[];
  loading: boolean;
}) {
  return (
    <Table
      data={profiles}
      columns={[
        {
          key: 'name',
          headerText: 'Profile Name',
        },
        {
          key: 'tags',
          headerText: 'Tags',
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
      <ClientSidePager
        nextPage={nextPage}
        prevPage={prevPage}
        data={data}
        {...pagination}
      />
    </>
  );
}

//  from e/web/teleport/src/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/ImportUserGroupsAndApps/FilterTable.tsx

const TableWrapper = styled.div`
  overflow-y: auto;
  border-bottom: 1px solid ${props => props.theme.colors.spotBackground[2]};
  height: 330px;
`;

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
    border-bottom: 1px solid ${props.theme.colors.spotBackground[2]};
    text-transform: none;
    padding: ${props.theme.space[2]}px 0;
    top: 0;
    position: sticky;
    z-index: 1;
    background-color: ${props.theme.colors.levels.surface};
    opacity: 1;
  }

  tbody > tr > td {
    padding: ${props.theme.space[3]}px 0;
  }

  tbody > tr {
    border: none;
  }
`
);
