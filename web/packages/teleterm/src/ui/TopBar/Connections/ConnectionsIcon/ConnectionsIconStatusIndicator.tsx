import React from 'react';
import styled from 'styled-components';
import { Box } from 'design';

export const ConnectionsIconStatusIndicator: React.FC<Props> = props => {
  const { connected, ...styles } = props;
  return <StyledStatus $connected={connected} {...styles} />;
};

const StyledStatus = styled<Props>(Box)`
  position: absolute;
  top: -4px;
  right: -4px;
  z-index: 1;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  box-shadow: 0 4px 8px rgba(0, 0, 0, 0.3);
  ${props => {
    const { $connected, theme } = props;
    const backgroundColor = $connected ? theme.colors.success : null;
    const border = $connected ? null : `1px solid ${theme.colors.light}`;
    return {
      backgroundColor,
      border,
    };
  }}
`;

type Props = {
  connected: boolean;
};
