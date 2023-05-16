/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import { Flex } from 'design';
import { useRefAutoFocus } from 'shared/hooks';

const Document: React.FC<{
  visible: boolean;
  onContextMenu?(): void;
  autoFocusDisabled?: boolean;
  [x: string]: any;
}> = ({ visible, children, onContextMenu, autoFocusDisabled, ...styles }) => {
  const ref = useRefAutoFocus<HTMLDivElement>({
    shouldFocus: visible && !autoFocusDisabled,
  });

  function handleContextMenu(
    e: React.MouseEvent<HTMLDivElement, MouseEvent>
  ): void {
    if (onContextMenu) {
      // `preventDefault` prevents opening the universal context menu
      // and thus only the document-specific menu gets displayed.
      // Opening two menus at the same time on Linux causes flickering.
      e.preventDefault();
      onContextMenu();
    }
  }

  return (
    <Flex
      tabIndex={visible ? 0 : -1}
      flex="1"
      ref={ref}
      bg="levels.sunken"
      onContextMenu={handleContextMenu}
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
