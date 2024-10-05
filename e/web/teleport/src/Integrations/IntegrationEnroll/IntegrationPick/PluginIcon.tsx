import React from 'react';
import styled from 'styled-components';

import { ResourceIcon } from 'design/ResourceIcon';

import { pluginMap } from '../PluginEnroll/plugins';

export function PluginIcon({ size, type, ...props }: Props) {
  const name = pluginMap[type]?.icon;
  if (!name) {
    return null;
  }
  return <Icon {...props} size={size} name={name} />;
}

interface Props {
  size?: number;
  type: string;
  [x: string]: any;
}

const Icon = styled(ResourceIcon)<{ size: number }>`
  display: inline-block;
  height: 100%;
  ${({ size }) =>
    size &&
    `
    max-height: ${size}px;
  `}
`;
