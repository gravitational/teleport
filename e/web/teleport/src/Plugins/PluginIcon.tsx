import React from 'react';
import styled from 'styled-components';

import { Image } from 'design';

import slackIcon from './assets/slack.svg';

const pluginIcons = {
  slack: slackIcon,
};

export function PluginIcon({ type }: Props) {
  const src = pluginIcons[type];
  if (!src) {
    return null;
  }
  return <Icon src={src} />;
}

type Props = {
  type: string;
};

const Icon = styled(Image)`
  padding-right: 8px;
  display: inline-block;
  height: 100%;
  max-height: 18px;
`;
