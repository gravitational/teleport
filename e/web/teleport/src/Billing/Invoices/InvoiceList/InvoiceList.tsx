import React, { useState } from 'react';
import styled from 'styled-components';
import { sortBy } from 'lodash';
import { Flex, LabelState, ButtonPrimary, Text } from 'design';
import {
  Cell,
  Column,
  TextCell,
  SortHeaderCell,
  SortTypes,
} from 'design/DataTable';
import PagedTable from 'design/DataTable/Paged';
import { ListItem } from './../useInvoices';

export default function Invoices(props: Props) {
  const [sort, setSort] = useState<Record<string, string>>({
    key: 'periodEnd',
    dir: SortTypes.DESC,
  });

  function onSortChange(key: string, dir: string) {
    setSort({ key, dir });
  }

  function sortAndFilter(invoices) {
    const sorted = sortBy(invoices, sort.key);
    if (sort.dir === SortTypes.DESC) {
      return sorted.reverse();
    }

    return sorted;
  }

  const data = sortAndFilter(props.invoices);
  const tableProps = { pageSize: 50, data };

  return (
    <Flex flexDirection="column" maxWidth="900px">
      <Text typography="h3" pb={3}>
        All Invoices
      </Text>
      <StyledTable {...tableProps}>
        <Column
          columnKey="status"
          cell={<StatusCell />}
          header={
            <SortHeaderCell
              sortDir={sort.key === 'status' ? sort.dir : null}
              onSortChange={onSortChange}
              title="Status"
            />
          }
        />
        <Column
          columnKey="amountDueText"
          cell={<TextCell />}
          header={<Cell>Amount Due</Cell>}
        />
        <Column
          columnKey="amountPaidText"
          cell={<TextCell />}
          header={<Cell>Amount Paid</Cell>}
        />
        <Column
          columnKey="periodEnd"
          cell={<PeriodCell />}
          header={
            <SortHeaderCell
              sortDir={sort.key === 'periodEnd' ? sort.dir : null}
              onSortChange={onSortChange}
              title="Period"
            />
          }
        />
        <Column header={<Cell />} cell={<ActionCell />} />
      </StyledTable>
    </Flex>
  );
}

const PeriodCell = props => {
  const { rowIndex, data } = props;
  const { periodText } = data[rowIndex] as ListItem;
  return <Cell>{periodText} </Cell>;
};

const StatusCell = props => {
  const { rowIndex, data } = props;
  const { status } = data[rowIndex] as ListItem;

  let kind = 'warning';
  if (status === 'PAID') {
    kind = 'success';
  }

  return (
    <Cell>
      <LabelState kind={kind} width="70px">
        {status}
      </LabelState>
    </Cell>
  );
};

const ActionCell = props => {
  const { rowIndex, data } = props;
  const { invoicePdf } = data[rowIndex] as ListItem;
  return (
    <Cell align="right">
      <ButtonPrimary
        as="a"
        target="_blank"
        href={invoicePdf}
        size="small"
        mr={3}
        width="80px"
      >
        Download
      </ButtonPrimary>
    </Cell>
  );
};

const StyledTable = styled(PagedTable)`
  tbody > tr > td {
    vertical-align: baseline;
  }
`;

type Props = {
  invoices: ListItem[];
};
