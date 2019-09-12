import React from 'react';
import styled from 'styled-components';
import { Box } from 'design';
import { StatusEnum } from 'teleport/services/clusters';

export default function Status({ status, ...styles }) {
  return <StyledStatus title={status} status={status} {...styles} />;
}

const StyledStatus = styled(Box)`
  width: 6px;
  height: 6px;
  border-radius: 50%;
  ${({ status, theme }) => {
    switch (status) {
      case StatusEnum.OFFLINE:
        return {
          backgroundColor: theme.colors.grey[300],
          boxShadow: `0px 0px 12px 4px ${theme.colors.grey[300]}`,
        };
      case StatusEnum.ONLINE:
        return {
          backgroundColor: theme.colors.success,
          boxShadow: `0px 0px 12px 4px ${theme.colors.success}`,
        };
      default:
        return {
          backgroundColor: theme.colors.warning,
          boxShadow: `0px 0px 12px 4px ${theme.colors.warning}`,
        };
    }
  }}
`;
