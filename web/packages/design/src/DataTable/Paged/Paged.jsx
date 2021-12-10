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
import { borderRadius, justifyContent } from 'design/system';
import InputSearch from 'teleport/components/InputSearch';
import { Table } from './../Table';
import Pager from './Pager';
import usePages from './usePages';
import PropTypes from 'prop-types';

export default function TablePaged(props) {
  const {
    pageSize,
    data,
    pagerPosition,
    fetchMore,
    fetchStatus,
    search,
    onSearchChange,
    ...rest
  } = props;
  const pagedState = usePages({ pageSize, data });
  const pagerProps = { ...pagedState, onFetch: fetchMore, fetchStatus };
  const tableProps = {
    ...rest,
    data: pagedState.data,
  };

  const showTopPager = !pagerPosition || pagerPosition === 'top';
  const showBottomPager = pagedState.hasPages || pagerPosition === 'bottom';
  const showSearchBar = !!onSearchChange;

  if (showBottomPager) {
    tableProps.borderBottomRightRadius = '0';
    tableProps.borderBottomLeftRadius = '0';
  } else {
    tableProps.borderTopRightRadius = '0';
    tableProps.borderTopLeftRadius = '0';
  }

  return (
    <div style={{ minWidth: 'min-content' }}>
      {showTopPager && (
        <StyledPanel
          borderTopRightRadius="3"
          borderTopLeftRadius="3"
          justifyContent={showSearchBar ? 'space-between' : 'end'}
        >
          {showSearchBar && (
            <InputSearch
              mr="3"
              onChange={onSearchChange}
              searchValue={search}
            />
          )}
          <Pager {...pagerProps} />
        </StyledPanel>
      )}
      <Table {...tableProps} />
      {showBottomPager && (
        <StyledPanel borderBottomRightRadius="3" borderBottomLeftRadius="3">
          <Pager {...pagerProps} />
        </StyledPanel>
      )}
    </div>
  );
}

TablePaged.propTypes = {
  pagerPosition: PropTypes.oneOf(['top', 'bottom']),
  fetchStatus: PropTypes.oneOf(['disabled', 'loading', '']),
};

export const StyledPanel = styled.nav`
  padding: 12px 16px;
  display: flex;
  height: 24px;
  flex-shrink: 0;
  align-items: center;
  justify-content: end;
  background: ${props => props.theme.colors.primary.light};
  ${borderRadius}
  ${justifyContent}
`;
