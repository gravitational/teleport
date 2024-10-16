import React from 'react';
import { Flex, ResourceIcon, Text } from 'design';

export const OktaBadge = () => {
  return (
    <Flex
      justifyContent="center"
      alignItems="center"
      gap={1}
      css={`
        background: ${props => props.theme.colors.spotBackground[0]};
        border-radius: 35px;
        width: 58px;
        height: 25px;
        color: ${p => p.theme.colors.text.slightlyMuted};
      `}
    >
      <ResourceIcon name="okta" height="16px" />
      <Text typography="body3">Okta</Text>
    </Flex>
  );
};
