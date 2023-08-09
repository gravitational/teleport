/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import styled from 'styled-components';

import { space, color, borderRadius } from 'design/system';

export function Icon({ size = 'medium', children, ...otherProps }: Props) {
  let iconSize = size;
  if (size === 'small') {
    iconSize = 16;
  }
  if (size === 'medium') {
    iconSize = 20;
  }
  if (size === 'large') {
    iconSize = 24;
  }
  if (size === 'extraLarge') {
    iconSize = 32;
  }
  return (
    <StyledIcon {...otherProps}>
      <svg
        fill="currentColor"
        height={iconSize}
        width={iconSize}
        viewBox="0 0 24 24"
      >
        {children}
      </svg>
    </StyledIcon>
  );
}

const StyledIcon = styled.span`
  display: inline-flex;
  align-items: center;
  justify-content: center;

  ${color};
  ${space};
  ${borderRadius};
`;

export type IconProps = {
  size?: 'small' | 'medium' | 'large' | 'extraLarge' | number;
  color?: string;
  title?: string;
  m?: number | string;
  mr?: number | string;
  ml?: number | string;
  mb?: number | string;
  mt?: number | string;
  my?: number | string;
  mx?: number | string;
  p?: number | string;
  pr?: number | string;
  pl?: number | string;
  pb?: number | string;
  pt?: number | string;
  py?: number | string;
  px?: number | string;
  role?: string;
  style?: React.CSSProperties;
  borderRadius?: number;
  onClick?: () => void;
  disabled?: boolean;
  as?: any;
  to?: string;
  className?: string;
};

type Props = IconProps & {
  children?: React.SVGProps<SVGPathElement> | React.SVGProps<SVGPathElement>[];
  a?: any;
};
