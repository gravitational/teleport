/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import React, { useRef } from 'react';

import ButtonIcon from 'design/ButtonIcon';
import { PushPin, PushPinFilled } from 'design/Icon';
import { HoverTooltip } from 'shared/components/ToolTip';

import { PinningSupport } from '../types';
import { PINNING_NOT_SUPPORTED_MESSAGE } from '../UnifiedResources';

export function PinButton({
  pinned,
  pinningSupport,
  hovered,
  setPinned,
  className,
}: {
  pinned: boolean;
  pinningSupport: PinningSupport;
  hovered: boolean;
  setPinned: () => void;
  className?: string;
}) {
  const copyAnchorEl = useRef(null);
  const tipContent = getTipContent(pinningSupport, pinned);

  const shouldShowButton =
    pinningSupport !== PinningSupport.Hidden && (pinned || hovered);
  const shouldDisableButton =
    pinningSupport === PinningSupport.Disabled ||
    pinningSupport === PinningSupport.NotSupported;

  const $content = pinned ? (
    <PushPinFilled color="brand" size="small" />
  ) : (
    <PushPin size="small" />
  );

  return (
    <ButtonIcon
      disabled={shouldDisableButton}
      setRef={copyAnchorEl}
      size={0}
      onClick={setPinned}
      className={className}
      css={`
        visibility: ${shouldShowButton ? 'visible' : 'hidden'};
        transition:
          color 0.3s,
          background 0.3s;
      `}
    >
      {tipContent && shouldShowButton ? (
        <HoverTooltip tipContent={tipContent}>{$content}</HoverTooltip>
      ) : (
        $content
      )}
      <HoverTooltip tipContent={tipContent}></HoverTooltip>
    </ButtonIcon>
  );
}

function getTipContent(
  pinningSupport: PinningSupport,
  pinned: boolean
): string {
  switch (pinningSupport) {
    case PinningSupport.NotSupported:
      return PINNING_NOT_SUPPORTED_MESSAGE;
    case PinningSupport.Supported:
      return pinned ? 'Unpin' : 'Pin';
    default:
      return '';
  }
}
