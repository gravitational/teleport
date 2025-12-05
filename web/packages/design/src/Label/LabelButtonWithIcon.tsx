import React from 'react';
import styled from 'styled-components';

import Flex from 'design/Flex';
import { IconProps } from 'design/Icon/Icon';

import Label, { LabelProps } from './Label';
import { IconPlacement } from './types';

export function LabelButtonWithIcon({
  Icon,
  placement,
  children,
  ...labelProps
}: LabelProps & {
  Icon: React.ComponentType<IconProps>;
  placement: IconPlacement;
  children: React.ReactNode;
  onClick?: () => void;
}) {
  const icon = (
    <ButtonIcon>
      <Icon size="small" />
    </ButtonIcon>
  );

  return (
    <LabelWithHoverAffect {...labelProps} withHoverState>
      <Flex gap={1} alignItems="center">
        {placement === 'left' && icon}
        {children}
        {placement === 'right' && icon}
      </Flex>
    </LabelWithHoverAffect>
  );
}

const ButtonIcon = styled.div`
  align-items: center;
  display: flex;
`;

const LabelWithHoverAffect = styled(Label)`
  &:hover {
    cursor: pointer;
  }
`;
