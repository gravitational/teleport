import React from 'react';
import styled from 'styled-components';
import { Box } from 'design';

export const ConnectionStatusIndicator: React.FC<Props> = props => {
  const { connected, ...styles } = props;
  return <StyledStatus $connected={connected} {...styles} />;
};

const StyledStatus = styled<Props>(Box)`
  width: 8px;
  height: 8px;
  border-radius: 50%;
  ${props => {
    const { $connected, theme } = props;
    const backgroundColor = $connected
      ? theme.colors.success
      : theme.colors.grey[300];
    return {
      backgroundColor,
    };
  }}
`;

type Props = {
  connected: boolean;
  [key: string]: any;
};
