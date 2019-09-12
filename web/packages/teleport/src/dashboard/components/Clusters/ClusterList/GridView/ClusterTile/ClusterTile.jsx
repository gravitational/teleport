import styled from 'styled-components';
import { keys } from 'lodash';
import React from 'react';
import { Flex, Box, Text, Card } from 'design';
import * as Icons from 'design/Icon';
import history from 'teleport/services/history';
import ActionMenu from './../../ClusterActionMenu';
import Status from '../../ClusterStatus';

export default function ClusterTile({ cluster, ...styles }) {
  const {
    url,
    clusterId,
    connectedText,
    labels,
    status,
    version,
    nodes = 1,
  } = cluster;
  const nodeText = `${nodes} - node${nodes === 1 ? '' : 's'}`;
  const refCard = React.useRef();
  const labelStr = keys(labels)
    .map(key => `${key}: ${labels[key]}`)
    .join(',');

  function onTileClick(e) {
    // ignore if text selection
    const selection = window.getSelection();
    if (selection.toString().length !== 0) {
      return;
    }

    // ignore if outside of card component
    if (!refCard.current.contains(e.target)) {
      return;
    }

    window.open(history.ensureBaseUrl(url), '_blank');
  }

  const statusText = `STATUS: ${status}`.toUpperCase();

  return (
    <StyledClusterTile
      tabIndex="0"
      ref={refCard}
      onClick={onTileClick}
      as={Flex}
      flexDirection="column"
      minHeight="300px"
      width="500px"
      {...styles}
    >
      <Flex
        borderTopLeftRadius="3"
        borderTopRightRadius="3"
        bg="primary.main"
        px="3"
        py="3"
        alignItems="center"
      >
        <Status status={status} mr="4" />
        <Box overflow="auto">
          <Text typography="h5" bold caps>
            {clusterId}
          </Text>
        </Box>
        <ActionMenu cluster={cluster} />
      </Flex>
      <Flex alignItems="center" flex="1" px="5">
        <LogoBox>
          <Icons.Cluster fontSize="36px" color="text.secondary" />
        </LogoBox>
        <Text typography="h6" bold color="text.primary">
          {statusText}
          <Text typography="body1" fontSize="1">
            CONNECTED: {connectedText}
            <Text>VERSION: {version}</Text>
          </Text>
        </Text>
      </Flex>
      <Flex
        borderBottomLeftRadius="3"
        borderBottomRightRadius="3"
        height="50px"
        bg="primary.main"
        color="text.primary"
        px="3"
        alignItems="center"
      >
        <Flex mr="5" alignItems="center" flex="0 0 auto">
          <Icons.Layers mr="2" />
          <Text typography="body2">{nodeText}</Text>
        </Flex>
        {labelStr && (
          <Flex alignItems="center" style={{ overflow: 'auto' }}>
            <Icons.Label mr="2" />
            <Text
              title={labelStr}
              typography="body2"
              style={{ whiteSpace: 'nowrap' }}
            >
              {labelStr}
            </Text>
          </Flex>
        )}
      </Flex>
    </StyledClusterTile>
  );
}

const StyledClusterTile = styled(Card)`
  cursor: pointer;
  outline: none;
  :hover,
  :focus {
    box-shadow: 0 24px 64px rgba(0, 0, 0, 0.56);
  }
`;

function LogoBox(props) {
  return (
    <Flex
      justifyContent="center"
      alignItems="center"
      bg="primary.dark"
      flexWrap="wrap"
      width="80px"
      height="80px"
      borderRadius="2"
      mr="3"
      flex="0 0 auto"
      {...props}
    />
  );
}
