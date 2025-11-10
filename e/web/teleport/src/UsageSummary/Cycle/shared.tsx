import styled from 'styled-components';

import { Flex } from 'design';

export function getPercentage({
  usage,
  limit,
  calibration,
}: {
  usage: number;
  limit: number;
  calibration: boolean;
}): number {
  if (calibration) {
    return 100;
  }
  return ~~Math.round((usage / limit) * 100);
}

export const CyclesContainer = styled(Flex)<{ enabled?: boolean }>`
  flex: 1 1 33%;
  min-width: 420px;
  background-color: ${({ theme }) => theme.colors.levels.surface};
  border-radius: 8px;
  padding: ${({ theme }) => theme.space[4]}px;
  flex-direction: column;
  justify-content: ${({ enabled }) => (enabled ? 'normal' : 'space-between')};
`;
