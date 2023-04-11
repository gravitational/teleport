import React from 'react';
import styled from 'styled-components';

import { Image } from 'design';

import { pluginTypeMap } from './data';

export function PluginIcon({ size, type, ...props }: Props) {
  const src = pluginTypeMap[type]?.icon;
  if (!src) {
    return null;
  }
  return <Icon {...props} size={size} src={src} />;
}

interface Props {
  size?: number;
  type: string;
  [x: string]: any;
}

const Icon = styled(Image)`
  display: inline-block;
  height: 100%;
  ${({ size }) =>
    size &&
    `
    max-height: ${size}px;
  `}
`;
