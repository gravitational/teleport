import React from 'react';
import styled from 'styled-components';
import { Text, Flex } from 'design';
import { Table, Cell, Column, TextCell } from 'design/DataTable';
import { BillingCycle, formatCents } from 'e-teleport/services/cloud';

export default function Cycle({ cycle, balance }: Props) {
  const { totalAmount, itemsList, periodStart, periodEnd } = cycle;
  const data = itemsList || [];
  const tableProps = { data };

  const periodTxt = `${new Date(
    periodStart * 1000
  ).toLocaleDateString()} - ${new Date(periodEnd * 1000).toLocaleDateString()}`;

  return (
    <Flex flexDirection="column">
      <Flex
        borderTopLeftRadius="2"
        borderTopRightRadius="2"
        justifyContent="space-between"
        alignItems="center"
        px={3}
        py={2}
        bg="primary.light"
      >
        <Text typography="h4" bold>
          Summary
        </Text>
        <Text typography="body2">Period: {periodTxt}</Text>
      </Flex>
      <StyledTable {...tableProps}>
        <Column
          columnKey="planDescription"
          cell={<TextCell />}
          header={<Cell>Description</Cell>}
        />
        <Column
          columnKey="quantity"
          cell={<TextCell />}
          header={<Cell>Quantity</Cell>}
        />
        <Column
          columnKey="amount"
          cell={<MoneyCell align="end" />}
          header={<Cell style={{ textAlign: 'end' }}>Amount</Cell>}
        />
      </StyledTable>
      <Text typography="h6" mr={4} mt={3} style={{ textAlign: 'end' }}>
        Total: {formatCents(totalAmount)}
      </Text>
    </Flex>
  );
}

type Props = {
  cycle: BillingCycle;
  balance: string;
};

const StyledTable = styled(Table)`
  tbody > tr > td {
    vertical-align: baseline;
  }
`;

const MoneyCell = props => {
  const { rowIndex, data, columnKey, ...rest } = props;
  const cents = data[rowIndex][columnKey];
  const txt = formatCents(cents);
  return <Cell {...rest}>{txt}</Cell>;
};
