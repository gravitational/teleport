import React from 'react';
import styled from 'styled-components';
import { Box, Text } from 'design';
import Table from 'design/DataTable';
import { MonthlyItem } from '../useUsage';

export default function UsageSummary({ items, mt = 0 }: Props) {
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
      <StyledTable
        data={items}
        columns={[
          {
            key: 'resource',
            headerText: 'SERVICES',
          },
          {
            key: 'jan',
            headerText: 'JAN',
          },
          {
            key: 'feb',
            headerText: 'FEB',
          },
          {
            key: 'mar',
            headerText: 'MAR',
          },
          {
            key: 'apr',
            headerText: 'APR',
          },
          {
            key: 'may',
            headerText: 'MAY',
          },
          {
            key: 'jun',
            headerText: 'JUN',
          },
          {
            key: 'jul',
            headerText: 'JUL',
          },
          {
            key: 'aug',
            headerText: 'AUG',
          },
          {
            key: 'sep',
            headerText: 'SEP',
          },
          {
            key: 'oct',
            headerText: 'OCT',
          },
          {
            key: 'nov',
            headerText: 'NOV',
          },
          {
            key: 'dec',
            headerText: 'DEC',
          },
        ]}
        emptyText="No Usage Data Found"
      />
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
` as typeof Table;

type Props = {
  items: MonthlyItem[];
  mt?: number;
};
