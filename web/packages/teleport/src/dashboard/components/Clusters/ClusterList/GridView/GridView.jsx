import React from 'react';
import styled from 'styled-components';
import { usePages } from 'design/DataTable/Paged';
import Pager, { StyledButtons } from 'design/DataTable/Paged/Pager';
import { Flex } from 'design';
import ClusterTile from './ClusterTile';

export default function GridView({ clusters, pageSize }) {
  const pagedState = usePages({ pageSize, data: clusters });
  const $clusters = pagedState.data.map(item => (
    <ClusterTile mr="5" mb="5" key={item.clusterId} cluster={item} />
  ));

  return (
    <>
      {pagedState.hasPages && (
        <Flex my="3" alignItems="center" justifyContent="space-between">
          <Pager {...pagedState} />
        </Flex>
      )}
      <Flex flexWrap="wrap">{$clusters}</Flex>
      {pagedState.hasPages && (
        <Flex my="3" alignItems="center" justifyContent="space-between">
          <Pager {...pagedState} />
        </Flex>
      )}
    </>
  );
}

export const StyledPanel = styled(Flex)`
  box-sizing: content-box;
  padding: 16px;
  height: 24px;
  background: ${props => props.theme.colors.primary.light};
  ${StyledButtons} {
    margin-left: ${props => `${props.theme.space[3]}px`};
  }
`;
