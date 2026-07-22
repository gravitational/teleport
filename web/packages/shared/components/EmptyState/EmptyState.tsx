/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import styled from 'styled-components';

import { Box, Flex, Text } from 'design';

export const FeatureContainer = styled(Flex)`
  @media (min-width: 1662px) {
    --feature-slider-width: 612px;
    --feature-width: 612px;
    --feature-height: 95px;
    --feature-preview-scale: scale(0.9);
    --feature-text-display: block;
  }

  @media (max-width: 1662px) {
    --feature-slider-width: 512px;
    --feature-width: 512px;
    --feature-height: 112px;
    --feature-preview-scale: scale(0.9);
    --feature-text-display: block;
  }

  @media (max-width: 1563px) {
    --feature-slider-width: 412px;
    --feature-width: 412px;
    --feature-height: 112px;
    --feature-preview-scale: scale(0.8);
    --feature-text-display: inline;
  }

  @media (max-width: 1462px) {
    --feature-slider-width: 412px;
    --feature-width: 412px;
    --feature-height: 112px;
    --feature-preview-scale: scale(0.8);
    --feature-text-display: inline;
  }

  @media (max-width: 1302px) {
    --feature-slider-width: 372px;
    --feature-width: 372px;
    --feature-height: 120px;
    --feature-preview-scale: scale(0.7);
    --feature-text-display: inline;
  }
`;

export const FeatureSlider = styled.div<{ $currIndex: number }>`
  z-index: -1;
  position: absolute;
  height: var(--feature-height);
  width: var(--feature-slider-width);

  transition: all 0.3s ease;
  border-radius: ${p => p.theme.radii[3]}px;
  cursor: pointer;

  top: calc(var(--feature-height) * ${p => p.$currIndex});

  background-color: ${p => p.theme.colors.interactive.tonal.primary[0]};
`;

export type FeatureProps = {
  isSliding: boolean;
  title: string;
  description: string;
  active: boolean;
  onClick(): void;
};

export const DetailsTab = ({
  active,
  onClick,
  isSliding,
  title,
  description,
}: FeatureProps) => {
  return (
    <Feature $active={active} onClick={onClick} $isSliding={isSliding}>
      <Title>{title}</Title>
      <Description>{description}</Description>
    </Feature>
  );
};

export const Title = styled(Text)`
  font-weight: bold;
`;

export const Description = styled(Text)`
  font-size: ${p => p.theme.fontSizes[1]}px;
`;

export const Feature = styled(Box)<{ $isSliding?: boolean; $active?: boolean }>`
  height: var(--feature-height);

  line-height: 20px;
  padding: ${p => p.theme.space[3]}px;
  border-radius: ${p => p.theme.radii[3]}px;
  cursor: pointer;
  width: var(--feature-width);

  background-color: ${p =>
    !p.$isSliding && p.$active
      ? p => p.theme.colors.interactive.tonal.primary[0]
      : 'inherit'};

  ${Title} {
    color: ${p => {
      if (p.$isSliding && p.$active) {
        return p.theme.colors.buttons.primary.default;
      }
      return p.$active ? p.theme.colors.buttons.primary.default : 'inherit';
    }};
    transition: color 0.2s ease-in 0s;
  }

  &:hover {
    background-color: ${p => p.theme.colors.spotBackground[0]};
  }

  &:hover ${Title} {
    color: ${p => p.theme.colors.text.main};
  }
`;
