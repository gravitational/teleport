import styled from 'styled-components';

import { Box, Flex, ResourceIcon } from 'design';

export function TerraformSideTab({
  onClick,
  panelWidth,
  top = 0,
}: {
  onClick(): void;
  panelWidth: number;
  top?: number | string;
}) {
  return (
    <TabWrapper position="absolute" top={top}>
      <Tab onClick={onClick} panelWidth={panelWidth}>
        <Flex justifyContent={'center'} alignItems={'center'} height={'100%'}>
          <ResourceIcon name="terraform" width="25px" height="25px" />
        </Flex>
      </Tab>
    </TabWrapper>
  );
}

const TabWrapper = styled(Box)`
  position: absolute;
  right: 40px;
`;

const Tab = styled(Box)<{ panelWidth: number }>`
  position: fixed;
  width: 40px;
  height: 40px;
  z-index: 100;
  border-top-left-radius: 6px;
  border-bottom-left-radius: 6px;
  background-color: ${p =>
    p.panelWidth ? p.theme.colors.spotBackground[0] : 'transparent'};

  &:hover {
    background-color: ${p => p.theme.colors.spotBackground[0]};
    cursor: pointer;
  }
`;
