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
import type { StyleFunction } from 'styled-components';

/**
 * outermostPadding is the padding used in the visually outermost layer of ClusterConnect.
 * Components inside should use spacing of value 3 or lower, otherwise individual elements might
 * appear too far from each other.
 */
export const outermostPadding = 4;

type NoExtraProps = Record<string, never>;
/**
 * dialogCss needs to avoid setting horizontal padding. This is so that child StepSlider components
 * in ClusterLogin are able to span the whole width of the parent component. This way when they
 * slide, they slide from one slide to another, without disappearing behind padding.
 */
export const dialogCss: StyleFunction<NoExtraProps> = props =>
  `
max-width: 480px;
width: 100%;
padding: ${props.theme.space[outermostPadding]}px 0 ${props.theme.space[outermostPadding]}px 0;
`;
