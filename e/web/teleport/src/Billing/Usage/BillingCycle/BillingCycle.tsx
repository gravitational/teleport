import React, { useState } from 'react';
import styled from 'styled-components';
import { Text, Flex, Card } from 'design';
import { displayUnixDate } from 'shared/services/loc';
import { Table, Cell, Column, TextCell } from 'design/DataTable';
import { BillingCycle, formatCents } from 'e-teleport/services/cloud';
import Select, { Option, DarkStyledSelect } from 'shared/components/Select';

export default function Cycle({ cycles, balance }: Props) {
  const [periodOptions] = useState<Option<BillingCycle>[]>(() => {
    const opts = cycles.map(c => {
      return {
        value: c,
        label: `${displayUnixDate(c.periodStart)}  -  ${displayUnixDate(
          c.periodEnd
        )}`,
      };
    });

    if (opts[0]) {
      // Mark active cycle with an asterik.
      // List of cycles come already sorted and first in array is the currently active cycle.
      opts[0].label = `${opts[0].label}*`;
    }

    return opts;
  });

  const [selectedPeriod, setSelectedPeriod] = useState<Option<BillingCycle>>(
    periodOptions[0]
  );
  const isCurrent =
    (selectedPeriod && selectedPeriod.value.state === 'started') ||
    !selectedPeriod;
  const data = selectedPeriod ? selectedPeriod.value.itemsList : [];
  const tableProps = { data };

  return (
    <Card>
      <Flex justifyContent="space-between" alignItems="center" px={3} py={2}>
        <Text typography="h4" bold>
          Summary
        </Text>
        {selectedPeriod && (
          <DarkStyledSelect width="208px">
            <Select
              value={selectedPeriod}
              onChange={(o: Option<BillingCycle>) => setSelectedPeriod(o)}
              options={periodOptions}
              isSearchable={false}
            />
          </DarkStyledSelect>
        )}
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
      <Flex
        py={3}
        pr={4}
        flexDirection="column"
        alignItems="flex-end"
        style={{ whiteSpace: 'pre' }}
      >
        {selectedPeriod && (
          <Text typography="body2">
            {`Total:    ${formatCents(selectedPeriod.value.totalAmount)}`}
          </Text>
        )}
        {isCurrent && (
          <Text
            mt={4}
            typography="h5"
            bold
          >{`Current Balance:    ${balance}`}</Text>
        )}
      </Flex>
    </Card>
  );
}

type Props = {
  cycles: BillingCycle[];
  balance: string;
};

const StyledTable = styled(Table)`
  box-shadow: none;

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
