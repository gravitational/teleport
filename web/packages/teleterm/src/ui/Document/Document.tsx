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

import React, { ReactNode, useState } from 'react';

import { Flex } from 'design';
import { useRefAutoFocus } from 'shared/hooks';

import { useIsInBackgroundMode } from 'teleterm/ui/hooks/useIsInBackgroundMode';

const Document: React.FC<{
  visible: boolean;
  autoFocusDisabled?: boolean;
  [x: string]: any;
}> = ({ visible, children, autoFocusDisabled, ...styles }) => {
  const ref = useRefAutoFocus<HTMLDivElement>({
    shouldFocus: visible && !autoFocusDisabled,
  });

  // The background-color of Document is controlled through <body> and it
  // cannot be set on Document directly because of Chromium issues with z-index.
  // Read more https://github.com/gravitational/teleport/pull/49351.
  return (
    <Flex
      data-testid={visible ? 'visible-doc' : ''}
      tabIndex={visible ? 0 : -1}
      flex="1"
      ref={ref}
      style={{
        overflow: 'auto',
        display: visible ? 'flex' : 'none',
        position: 'relative',
        outline: 'none',
      }}
      {...styles}
    >
      {children}
    </Flex>
  );
};

export default Document;

/**
 * Wrapper for sessions that should end when the app is in background mode.
 *
 * When `connected` and the window goes into the background, this component
 * unmounts its children, terminating any session tied to the document
 * (e.g. desktop or SSH). The children are restored when the window
 * becomes visible again and `visible` is true.
 */
export function ForegroundSession({
  connected,
  visible,
  children,
}: {
  /** When `true`, children are unmounted if the app is in the background. */
  connected: boolean;
  /** When `true`, children are mounted. */
  visible: boolean;
  children: ReactNode;
}) {
  const isInBackgroundMode = useIsInBackgroundMode();
  if (isInBackgroundMode && connected) {
    return;
  }

  return (
    <MountWhenVisible visible={!isInBackgroundMode && visible}>
      {children}
    </MountWhenVisible>
  );
}

/** Defers mounting the children until they are visible. */
function MountWhenVisible({
  visible,
  children,
}: {
  visible: boolean;
  children: ReactNode;
}) {
  const [showChildren, setShowChildren] = useState(visible);

  if (!showChildren && visible) {
    setShowChildren(true);
  }

  return showChildren ? children : undefined;
}
