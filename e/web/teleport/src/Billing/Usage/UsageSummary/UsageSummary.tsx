import React from 'react';
import styled from 'styled-components';
import { Box, Text } from 'design';
import { Cell, Column, TextCell, Table } from 'design/DataTable';
import { MonthlyItem } from '../useUsage';

export default function UsageSummary({ items, mt = 0 }: Props) {
  const tableProps = { data: items };
  return (
    <Box maxWidth="900px" mt={mt}>
      <Box
        px={3}
        py={2}
        bg="primary.light"
        borderTopRightRadius={3}
        borderTopLeftRadius={3}
      >
        <Text typography="h4" bold>
          Usage Summary
        </Text>
      </Box>
      <StyledTable {...tableProps}>
        <Column
          columnKey="resource"
          cell={<TextCell />}
          header={<Cell>SERVICES</Cell>}
        />
        <Column columnKey="jan" cell={<TextCell />} header={<Cell>JAN</Cell>} />
        <Column columnKey="feb" cell={<TextCell />} header={<Cell>FEB</Cell>} />
        <Column columnKey="mar" cell={<TextCell />} header={<Cell>MAR</Cell>} />
        <Column columnKey="apr" cell={<TextCell />} header={<Cell>APR</Cell>} />
        <Column columnKey="may" cell={<TextCell />} header={<Cell>MAY</Cell>} />
        <Column columnKey="jun" cell={<TextCell />} header={<Cell>JUN</Cell>} />
        <Column columnKey="jul" cell={<TextCell />} header={<Cell>JUL</Cell>} />
        <Column columnKey="aug" cell={<TextCell />} header={<Cell>AUG</Cell>} />
        <Column columnKey="sep" cell={<TextCell />} header={<Cell>SEP</Cell>} />
        <Column columnKey="oct" cell={<TextCell />} header={<Cell>OCT</Cell>} />
        <Column columnKey="nov" cell={<TextCell />} header={<Cell>NOV</Cell>} />
        <Column columnKey="dec" cell={<TextCell />} header={<Cell>DEC</Cell>} />
      </StyledTable>
    </Box>
  );
}

const StyledTable = styled(Table)`
  thead > tr > th:not(:first-child) {
    text-align: center;
  }

  tbody > tr > td {
    text-align: center;

    :first-child {
      background-color: ${({ theme }) => theme.colors.primary.lighter};
      padding-top: 16px;
      padding-bottom: 16px;
      text-align: left;
    }
  }
`;

type Props = {
  items: MonthlyItem[];
  mt?: number;
};
