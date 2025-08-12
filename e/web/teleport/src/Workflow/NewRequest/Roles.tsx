import { useEffect, useState } from 'react';

import { ButtonBorder, ButtonPrimary } from 'design/Button';
import Table, { Cell } from 'design/DataTable';
import { SearchPanel } from 'shared/components/Search';

import { useServerSidePagination } from 'teleport/components/hooks';
import ResourceService, { RequestableRole } from 'teleport/services/resources';

export function Roles({
  fetchFunc,
  requested,
  onToggleRole,
  allRequestableRoles,
}: {
  fetchFunc: typeof ResourceService.prototype.fetchRequestableRoles;
  requested: Set<string>;
  onToggleRole(role: string): void;
  allRequestableRoles: string[];
}) {
  const [search, setSearch] = useState('');
  const serverSidePagination = useServerSidePagination<RequestableRole>({
    pageSize: 20,
    fetchFunc: async (_, params) => {
      const { items, startKey } = await fetchFunc(params, allRequestableRoles);
      return { agents: items || [], startKey, totalCount: items?.length || 0 };
    },
    clusterId: '',
    params: { search },
  });

  useEffect(() => {
    serverSidePagination.fetch();
  }, [search]);

  const addToRequestText = requested.size
    ? '+ Add to Request'
    : '+ Request Access';

  return (
    <Table
      data={serverSidePagination.fetchedData.agents}
      fetching={{
        fetchStatus: serverSidePagination.fetchStatus,
        onFetchNext: serverSidePagination.fetchNext,
        onFetchPrev: serverSidePagination.fetchPrev,
      }}
      serversideProps={{
        sort: undefined,
        setSort: () => undefined,
        serversideSearchPanel: (
          <SearchPanel
            updateSearch={setSearch}
            updateQuery={null}
            hideAdvancedSearch={true}
            filter={{ search }}
            disableSearch={serverSidePagination.attempt.status === 'processing'}
          />
        ),
      }}
      pagination={{
        pagerPosition: 'top',
        pageSize: serverSidePagination.pageSize,
      }}
      isSearchable={true}
      columns={[
        {
          key: 'name',
          headerText: 'Role Name',
        },
        {
          key: 'description',
          headerText: 'Description',
        },
        {
          altKey: 'action',
          render: ({ name }) => {
            const isAdded = requested.has(name);
            const commonProps = {
              width: '137px',
              size: 'small' as const,
              onClick: () => onToggleRole(name),
            };
            return (
              <Cell align="right">
                {isAdded ? (
                  <ButtonPrimary {...commonProps}>Remove</ButtonPrimary>
                ) : (
                  <ButtonBorder {...commonProps}>
                    {addToRequestText}
                  </ButtonBorder>
                )}
              </Cell>
            );
          },
        },
      ]}
      emptyText="No Requestable Roles Found"
    />
  );
}
