import React, { PropsWithChildren } from 'react';
import styled from 'styled-components';

import Flex from 'design/Flex';
import { IconProps } from 'design/Icon/Icon';

import Label, { LabelProps } from './Label';

export function LabelButtonWithIcon({
  IconLeft,
  IconRight,
  children,
  title,
  withHoverState = false,
  ...labelProps
}: {
  IconLeft?: React.ComponentType<IconProps>;
  IconRight?: React.ComponentType<IconProps>;
  onClick?: () => void;
  title?: string;
  withHoverState?: boolean;
} & LabelProps &
  PropsWithChildren) {
  const Icon = IconLeft ?? IconRight;

  let icon;
  if (Icon) {
    icon = (
      <ButtonIcon>
        <Icon size="small" />
      </ButtonIcon>
    );
  }

  return (
    <LabelWithHoverAffect
      {...labelProps}
      title={title}
      withHoverState={withHoverState}
      tabIndex={0}
    >
      <Flex gap={1} alignItems="center">
        {IconLeft && icon}
        {children}
        {IconRight && icon}
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
    cursor: ${p => (p.withHoverState ? 'pointer' : 'default')};
  }
`;
