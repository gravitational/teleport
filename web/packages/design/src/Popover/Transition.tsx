/*
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

import React, { forwardRef, useImperativeHandle, useLayoutEffect } from 'react';

import { useResizeObserverRef } from 'design/utils/useResizeObserverRef';

/**
 * Transition is a helper for firing certain effects from Popover, as it's way easier to use them
 * this way than integrating with the component lifecycle.
 */
export const Transition = forwardRef<
  { resizeObserverRef: React.RefCallback<HTMLElement> },
  React.PropsWithChildren<{
    onEntering: () => void;
    onPaperResize: () => void;
  }>
>(({ onEntering, onPaperResize, children }, imperativeRef) => {
  // Note: useLayoutEffect to prevent flickering improperly positioned popovers.
  // It's especially noticeable on Safari.
  useLayoutEffect(onEntering, []);

  const resizeObserverRef = useResizeObserverRef(onPaperResize);
  useImperativeHandle(
    imperativeRef,
    () => ({
      resizeObserverRef,
    }),
    [resizeObserverRef]
  );

  return children;
});
