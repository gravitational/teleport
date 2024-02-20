/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useRef } from 'react';

import { PushPinFilled, PushPin } from 'design/Icon';
import ButtonIcon from 'design/ButtonIcon';

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
