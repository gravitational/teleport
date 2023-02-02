import React from 'react';
import styled from 'styled-components';

import { Box, Flex, Text } from 'design';

import * as Icons from 'design/Icon';

import { TextIcon } from 'teleport/Discover/Shared/Text';

const HintBoxContainer = styled(Box)`
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

interface HintBoxProps {
  header: string;
}

export function HintBox(props: React.PropsWithChildren<HintBoxProps>) {
  return (
    <HintBoxContainer>
      <Text color="warning">
        <Flex alignItems="center" mb={2}>
          <TextIcon
            color="warning"
            css={`
              white-space: pre;
            `}
          >
            <Icons.Warning fontSize={4} color="warning" />
          </TextIcon>
          {props.header}
        </Flex>
      </Text>

      {props.children}
    </HintBoxContainer>
  );
}
