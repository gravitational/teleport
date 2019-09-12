/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import styled from 'styled-components';
import { borderRadius } from 'design/system';
import { Table } from 'design/DataTable';
import { Flex } from 'design';
import { usePages } from 'design/DataTable/Paged';
import Pager, { StyledButtons } from 'design/DataTable/Paged/Pager';
import FieldInputSsh from './FieldInputSsh';

export default function PagedTable(props) {
  const { pageSize, data, ...rest } = props;
  const pagedState = usePages({ pageSize, data });
  const tableProps = {
    ...rest,
    data: pagedState.data,
  };

  const { hasPages } = pagedState;

  if (hasPages) {
    tableProps.borderBottomRightRadius = '0';
    tableProps.borderBottomLeftRadius = '0';
  }

  return (
    <div>
      <StyledPanel
        alignItems="center"
        borderTopRightRadius="3"
        borderTopLeftRadius="3"
        justifyContent="space-between"
      >
        <FieldInputSsh />
        {hasPages && (
          <Flex alignItems="center" justifyContent="flex-end">
            <Pager {...pagedState} />
          </Flex>
        )}
      </StyledPanel>
      <StyledTable {...tableProps} />
      {hasPages && (
        <StyledPanel
          alignItems="center"
          justifyContent="space-between"
          borderBottomRightRadius="3"
          borderBottomLeftRadius="3"
        >
          <Pager {...pagedState} />
        </StyledPanel>
      )}
    </div>
  );
}

export const StyledPanel = styled(Flex)`
  box-sizing: content-box;
  padding: 16px;
  height: 24px;
  background: ${props => props.theme.colors.primary.light};
  ${borderRadius}
  ${StyledButtons} {
    margin-left: ${props => `${props.theme.space[3]}px`};
  }
`;

const StyledTable = styled(Table)`
  & > tbody > tr > td {
    vertical-align: baseline;
  }
`;
