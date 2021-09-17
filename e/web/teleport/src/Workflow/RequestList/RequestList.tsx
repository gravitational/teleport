import React, { useState } from 'react';
import { Link } from 'react-router-dom';
import styled from 'styled-components';
import { sortBy } from 'lodash';
import {
  Flex,
  Text,
  Label,
  LabelState,
  ButtonBorder,
  ButtonPrimary,
  Box,
  Alert,
  Indicator,
} from 'design';
import { Cell, Column, SortHeaderCell, SortTypes } from 'design/DataTable';
import PagedTable from 'design/DataTable/Paged';
import isMatch from 'design/utils/match';
import InputSearch from 'teleport/components/InputSearch';
import useTeleportE from 'e-teleport/useTeleportE';
import useRequestList, { State, Row } from './useRequestList';
import cfg from 'e-teleport/config';

export default function Container() {
  const ctx = useTeleportE();
  const state = useRequestList(ctx);
  return <RequestList {...state} />;
}

export function RequestList({ attempt, requests, assumeRole }: State) {
  // Delaying indicator is default behavior.
  // This flag is used to show indicator immedidately after
  // user clicks "assume" button, which removes the awkward blank
  // moment where nothing seems to be happening for
  // the duration of the default delay moment.
  const [delayIndicator, setDelayIndicator] = useState(true);
  const [searchValue, setSearchValue] = useState('');
  const [sort, setSort] = useState<Record<string, string>>({
    key: 'created',
    dir: SortTypes.DESC,
  });

  function onSortChange(key: string, dir: string) {
    setSort({ key, dir });
  }

  function onSearchChange(value: string) {
    setSearchValue(value);
  }

  function sortAndFilter(searchValue: string) {
    const searchableProps = [
      'user',
      'roles',
      'requestReason',
      'createdDuration',
      'state',
    ];
    const filtered = requests.filter(req =>
      isMatch(req, searchValue, { searchableProps, cb: null })
    );

    // Apply sorting to filtered list.
    const sorted = sortBy(filtered, sort.key);
    if (sort.dir === SortTypes.DESC) {
      return sorted.reverse();
    }

    return sorted;
  }

  function onAssumeRole(request: Row) {
    setDelayIndicator(false);
    assumeRole(request);
  }

  const data = requests ? sortAndFilter(searchValue) : [];
  const tableProps = { pageSize: 20, data };

  return (
    <>
      {attempt.status === 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator delay={delayIndicator ? 'short' : 'none'} />
        </Box>
      )}
      {attempt.status === 'failed' && (
        <Alert kind="danger" children={attempt.statusText} />
      )}
      {attempt.status === 'success' && (
        <>
          <Flex flex="0 0 auto" mb={4} alignItems="center">
            <InputSearch onChange={onSearchChange} />
          </Flex>
          <StyledTable {...tableProps}>
            <Column
              columnKey="state"
              cell={<StatusCell />}
              header={
                <SortHeaderCell
                  sortDir={sort.key === 'state' ? sort.dir : null}
                  onSortChange={onSortChange}
                  title="Status"
                />
              }
            />
            <Column
              columnKey="user"
              cell={<UserCell />}
              header={
                <SortHeaderCell
                  sortDir={sort.key === 'user' ? sort.dir : null}
                  onSortChange={onSortChange}
                  title="User"
                />
              }
            />
            <Column
              columnKey="roles"
              cell={<RolesCell />}
              header={
                <SortHeaderCell
                  sortDir={sort.key === 'roles' ? sort.dir : null}
                  onSortChange={onSortChange}
                  title="Roles"
                />
              }
            />
            <Column
              columnKey="requestReason"
              cell={<ReasonCell />}
              header={
                <SortHeaderCell
                  sortDir={sort.key === 'requestReason' ? sort.dir : null}
                  onSortChange={onSortChange}
                  title="Request Reason"
                />
              }
            />
            <Column
              columnKey="created"
              cell={<CreatedCell />}
              header={
                <SortHeaderCell
                  sortDir={sort.key === 'created' ? sort.dir : null}
                  onSortChange={onSortChange}
                  title="Created"
                />
              }
            />
            <Column
              header={<Cell />}
              cell={<ActionCell assumeRole={onAssumeRole} />}
            />
          </StyledTable>
        </>
      )}
    </>
  );
}

const UserCell = props => {
  const { rowIndex, data } = props;
  const { user } = data[rowIndex] as Row;

  return (
    <Cell
      style={{
        maxWidth: '100px',
        whiteSpace: 'nowrap',
        overflow: 'hidden',
        textOverflow: 'ellipsis',
      }}
      title={user}
    >
      {user}
    </Cell>
  );
};

const ReasonCell = props => {
  const { rowIndex, data } = props;
  const { requestReason } = data[rowIndex] as Row;

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

const CreatedCell = props => {
  const { rowIndex, data } = props;
  const { createdDuration } = data[rowIndex] as Row;
  return <Cell>{createdDuration}</Cell>;
};

const StatusCell = props => {
  const { rowIndex, data } = props;
  const { state } = data[rowIndex] as Row;

  let kind = 'warning';
  if (state === 'APPROVED') {
    kind = 'success';
  } else if (state === 'DENIED') {
    kind = 'danger';
  }

  return (
    <Cell style={{ display: 'flex', alignItems: 'center' }}>
      <LabelState
        kind={kind}
        mr={2}
        width="10px"
        p={0}
        style={{ minHeight: '10px' }}
      />
      <Text typography="body2">{state}</Text>
    </Cell>
  );
};

const ActionCell = props => {
  const { rowIndex, data, assumeRole } = props;
  const request = data[rowIndex] as Row;

  return (
    <Cell align="right" style={{ whiteSpace: 'nowrap' }}>
      {request.canAssume && (
        <ButtonPrimary
          size="small"
          disabled={request.isAssumed}
          onClick={() => assumeRole(request)}
          width="108px"
        >
          {request.isAssumed ? 'assumed' : 'assume roles'}
        </ButtonPrimary>
      )}
      <ButtonBorder
        as={Link}
        size="small"
        ml={3}
        to={cfg.getAccessRequestRoute(request.id)}
      >
        View
      </ButtonBorder>
    </Cell>
  );
};

const RolesCell = props => {
  const { rowIndex, data } = props;
  const { roles } = data[rowIndex] as Row;
  const $roles = roles.sort().map(role => (
    <Label mb="0" mr="1" key={role} kind="secondary">
      {role}
    </Label>
  ));

  return <Cell>{$roles}</Cell>;
};

const StyledTable = styled(PagedTable)`
  tbody > tr > td {
    vertical-align: baseline;
  }
`;
