/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { type Placement } from '@floating-ui/react';
import React, { PropsWithChildren } from 'react';

import Flex from 'design/Flex';
import { BaseTooltip } from 'design/Tooltip/shared';

type HoverTooltipProps = {
  /**
   * String or ReactNode to display in tooltip.
   */
  tipContent?: React.ReactNode;
  /**
   * Only show tooltip if trigger content is overflowing its container.
   */
  showOnlyOnOverflow?: boolean;
  /**
   * Element's class name. Might seem unimportant, but required for using the
   * styled-components' `css` property.
   */
  className?: string;
  /**
   * Show arrow on the tooltip.
   */
  arrow?: boolean;
  /**
   * Specifies the position of tooltip relative to trigger content.
   */
  placement?: Placement;
  /**
   * @deprecated – Prefer specifying `placement` instead.
   */
  position?: Placement;
  /**
   * Offset the tooltip relative to trigger content. Defaults to `8`.
   */
  offset?: number;
  /**
   * Delay opening and/or closing of the tooltip.
   */
  delay?: number | { open: number; close: number };
  /**
   * Flip the tooltip's placement when tooltip runs out of the viewport.
   */
  flip?: boolean;
  /**
   * Transition the tooltip in/out on mount/unmount.
   */
  animate?: boolean;
};

export const HoverTooltip = ({
  tipContent,
  children,
  showOnlyOnOverflow = false,
  position,
  placement = 'top',
  arrow = true,
  ...tooltipProps
}: PropsWithChildren<HoverTooltipProps>) => (
  <BaseTooltip
    content={tipContent}
    placement={position || placement}
    onlyOnOverflow={showOnlyOnOverflow}
    arrow={arrow}
    {...tooltipProps}
  >
    <Flex>{children}</Flex>
  </BaseTooltip>
);
