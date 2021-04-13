import React, { useState } from 'react';
import styled from 'styled-components';
import { Text, Box, Flex, Card } from 'design';
import { displayUnixDate } from 'shared/services/loc';
import { Table, Cell, Column, TextCell } from 'design/DataTable';
import { BillingCycle, formatCents } from 'e-teleport/services/cloud';
import Select, { Option } from 'shared/components/Select';

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
          <SelectContainer width="208px">
            <Select
              value={selectedPeriod}
              onChange={(o: Option<BillingCycle>) => setSelectedPeriod(o)}
              options={periodOptions}
              width="230px"
              isSearchable={false}
            />
          </SelectContainer>
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

const SelectContainer = styled(Box)(
  ({ theme }) => `
  .react-select__control,
  .react-select__control--is-focused {
    border-color: #FFF;
    height: 34px;
    min-height: 34px;
  }

  .react-select__option {
    padding: 4px 12px;
  }
  .react-select__option--is-focused,
  .react-select__option--is-focused:active {
    background-color: ${theme.colors.grey[50]};
  }

  .react-select__menu {
    margin-top: 0px;
    font-size: 14px;
  }

  react-select__menu-list {
  }

  .react-select__indicator-separator {
    display: none;
  }

  .react-select__value-container{
    height: 30px;
    padding: 0 8px;
  }

  .react-select__option--is-selected {
    background-color: inherit;
    color: inherit;
  }

  .react-select__option--is-focused {
    background-color: #cfd8dc;
    color: inherit;
  }

  .react-select__single-value{
    color: white;
    font-size: 14px;
    width: 270px;
    text-overflow: ellipsis;
  }

  .react-select__dropdown-indicator{
    padding: 4px 8px;
    color: ${theme.colors.text.secondary};
  }

  input {
    font-family: ${theme.font};
    font-size: 14px;
    height: 26px;
  }

  .react-select__input {
    color: white;
    height: 20px;
    font-size: 14px;
    font-family: ${theme.font};
  }

  .react-select__control {
    border-radius: 4px;
    border-color: rgba(255, 255, 255, 0.24);
    background-color: ${theme.colors.primary.dark};
    color: ${theme.colors.text.secondary};

    &:focus, &:active {
      background-color: ${theme.colors.primary.lighter};
    }

    &:hover {
      border-color: rgba(255, 255, 255, 0.24);
      background-color: ${theme.colors.primary.lighter};
      .react-select__dropdown-indicator{
        color: ${theme.colors.text.primary};
      }
    }
  }

  .react-select__control--is-focused {
    background-color: ${theme.colors.primary.lighter};
    border-color: transparent;
    border-radius: 4px;
    border-style: solid;
    border-width: 1px;
    box-shadow: none;
    border-color: rgba(255, 255, 255, 0.24);

    .react-select__dropdown-indicator{
      color: ${theme.colors.text.secondary};
    }
  }

  .react-select__menu {
    border-top-left-radius: 0;
    border-top-right-radius: 0;
  }

  .react-select__loading-indicator{
    display: none;
  }
`
);
