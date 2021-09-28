import React, { useState } from 'react';
import styled from 'styled-components';
import { sortBy } from 'lodash';
import { ButtonBorder, Text } from 'design';
import {
  Column,
  SortHeaderCell,
  Cell,
  TextCell,
  SortTypes,
  Table,
} from 'design/DataTable';
import { displayDate } from 'shared/services/loc';
import { MfaDevice } from 'teleport/services/mfa/types';

export default function MfaDeviceList({
  devices = [],
  remove,
  mostRecentDevice,
  ...styles
}: Props) {
  const [sortDir, setSortDir] = useState<Record<string, string>>({
    registeredDate: SortTypes.ASC,
  });

  function sort(data) {
    const columnKey = Object.getOwnPropertyNames(sortDir)[0];
    const sorted = sortBy(data, columnKey);
    if (sortDir[columnKey] === SortTypes.ASC) {
      return sorted.reverse();
    }

    return sorted;
  }

  function onSortChange(columnKey: string, sortDir: string) {
    setSortDir({ [columnKey]: sortDir });
  }

  const data = sort(devices);

  return (
    <StyledTable data={data} {...styles}>
      <Column
        columnKey="description"
        cell={<TextCell />}
        header={<Cell>Type</Cell>}
      />
      <Column
        columnKey="name"
        cell={<NameCell />}
        header={<Cell>Device Name</Cell>}
      />
      <Column
        columnKey="registeredDate"
        header={
          <SortHeaderCell
            sortDir={sortDir.registeredDate}
            onSortChange={onSortChange}
            title="Registered"
          />
        }
        cell={<DateCell />}
      />
      <Column
        columnKey="lastUsedDate"
        header={
          <SortHeaderCell
            sortDir={sortDir.lastUsedDate}
            onSortChange={onSortChange}
            title="Last Used"
          />
        }
        cell={<DateCell />}
      />
      <Column
        header={<Cell />}
        cell={
          <RemoveCell remove={remove} mostRecentDevice={mostRecentDevice} />
        }
      />
    </StyledTable>
  );
}

const NameCell = props => {
  const { data, rowIndex } = props;
  const { name } = data[rowIndex];

  return (
    <Cell title={name}>
      <Text
        style={{
          maxWidth: '96px',
          whiteSpace: 'nowrap',
        }}
      >
        {name}
      </Text>
    </Cell>
  );
};

const DateCell = props => {
  const { data, rowIndex, columnKey } = props;
  const dateText = displayDate(data[rowIndex][columnKey]);

  return <Cell>{dateText}</Cell>;
};

const RemoveCell = props => {
  const { data, rowIndex, remove, mostRecentDevice } = props;
  const { id, name } = data[rowIndex];

  if (id === mostRecentDevice?.id) {
    return null;
  }

  return (
    <Cell align="right">
      <ButtonBorder size="small" onClick={() => remove({ id, name })}>
        Remove
      </ButtonBorder>
    </Cell>
  );
};

type Props = {
  devices: MfaDevice[];
  remove({ id, name }: { id: string; name: string }): void;
  mostRecentDevice?: MfaDevice;
  [key: string]: any;
};

const StyledTable = styled(Table)`
  & > tbody > tr {
    td {
      vertical-align: middle;
      height: 32px;
    }
  }
`;
