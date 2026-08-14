import Table, { Cell } from 'design/DataTable';
import { requestMatcher } from 'shared/components/AccessRequests/NewRequest/matcher';
import {
  renderIdCell,
  renderStatusCell,
  renderUserCell,
} from 'shared/components/AccessRequests/ReviewRequests';
import { AccessRequest } from 'shared/services/accessRequests';

import { RequestedCell } from 'e-teleport/Workflow/ReviewRequests/RequestList/RequestList';
import { AccessRequestWithFlags } from 'e-teleport/Workflow/ReviewRequests/RequestList/useRequestList';
import {
  renderActionCell,
  SimpleListProps,
} from 'teleport/LocksV2/NewLock/ResourceList/common';

export function AccessRequests(
  props: SimpleListProps & { requests: AccessRequest[] }
) {
  const {
    requests = [],
    selectedResources,
    toggleSelectResource,
    fetchStatus,
    pageSize,
  } = props;

  return (
    <Table
      data={requests}
      columns={[
        {
          key: 'id',
          headerText: 'Id',
          isSortable: true,
          render: renderIdCell,
        },
        {
          key: 'state',
          headerText: 'Status',
          isSortable: true,
          render: renderStatusCell,
        },
        {
          key: 'user',
          headerText: 'User',
          isSortable: true,
          render: renderUserCell,
        },
        {
          key: 'roles',
          headerText: 'Requested',
          render: ({ resources, roles, id }) => (
            <RequestedCell resources={resources} roles={roles} id={id} />
          ),
        },
        {
          key: 'resources',
          isNonRender: true,
        },
        {
          key: 'requestReason',
          headerText: 'Request Reason',
          isSortable: true,
          render: renderReasonCell,
        },
        {
          key: 'created',
          headerText: 'Created',
          isSortable: true,
          render: ({ createdDuration }) => <Cell>{createdDuration}</Cell>,
        },
        {
          altKey: 'action-btn',
          render: ({ id }) =>
            renderActionCell(
              Boolean(selectedResources.access_request[id]),
              () =>
                toggleSelectResource({
                  kind: 'access_request',
                  targetValue: id,
                })
            ),
        },
      ]}
      emptyText="No Requests Found"
      isSearchable
      pagination={{ pageSize }}
      initialSort={{ key: 'created', dir: 'DESC' }}
      customSearchMatchers={[requestMatcher]}
      fetching={{
        fetchStatus,
      }}
    />
  );
}

const renderReasonCell = ({ requestReason }: AccessRequestWithFlags) => {
  return (
    <Cell
      style={{
        maxWidth: '150px',
        whiteSpace: 'nowrap',
        overflow: 'hidden',
        textOverflow: 'ellipsis',
      }}
      title={requestReason}
    >
      {requestReason}
    </Cell>
  );
};
