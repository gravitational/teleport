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
import { Table, Column, Cell, TextCell } from 'design/DataTable';
import { usePages, Pager, StyledPanel } from 'design/DataTable/Paged';
import InputSearch from '../InputSearch';
import { Flex } from 'design';
import { borderRadius } from 'design/system';

export default function SessionList(props) {
  const { pageSize, events, ...rest } = props;
  const pagedState = usePages({ pageSize, data: events });
  const tableProps = {
    ...rest,
    data: pagedState.data,
  };

  return (
    <>
      <BorderedFlex
        bg="primary.light"
        py="3"
        px="3"
        borderTopRightRadius="3"
        borderTopLeftRadius="3"
      >
        <InputSearch />
      </BorderedFlex>
      {pagedState.hasPages && (
        <StyledPanel>
          <Pager {...pagedState} />
        </StyledPanel>
      )}
      <Table {...tableProps}>
        <Column
          columnKey="user"
          header={<Cell>Description</Cell>}
          cell={<TextCell />}
        />
        <Column
          columnKey="id"
          header={<Cell>Description</Cell>}
          cell={<TextCell />}
        />
      </Table>
    </>
  );
}

const BorderedFlex = styled(Flex)`
  ${borderRadius}
`;
