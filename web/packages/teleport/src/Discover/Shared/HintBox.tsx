import styled from 'styled-components';

import { Box } from 'design';

export const HintBox = styled(Box)`
  max-width: 1000px;
  background-color: rgba(255, 255, 255, 0.05);
  padding: ${props => `${props.theme.space[3]}px`};
  border-radius: ${props => `${props.theme.space[2]}px`};
  border: 2px solid ${props => props.theme.colors.warning}; ;
`;

export const WaitingInfo = styled(Box)`
  max-width: 1000px;
  background-color: rgba(255, 255, 255, 0.05);
  padding: ${props => `${props.theme.space[3]}px`};
  border-radius: ${props => `${props.theme.space[2]}px`};
  border: 2px solid #2f3659;
  display: flex;
  align-items: center;
`;
