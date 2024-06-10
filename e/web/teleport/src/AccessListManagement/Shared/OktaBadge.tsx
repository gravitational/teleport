import React from 'react';
import { Flex } from 'design';
import { OktaIcon } from 'design/SVGIcon';

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
        font-size: ${p => p.theme.fontSizes[1]}px;
        color: ${p => p.theme.colors.text.slightlyMuted};
      `}
    >
      <OktaIcon />
      Okta
    </Flex>
  );
};
