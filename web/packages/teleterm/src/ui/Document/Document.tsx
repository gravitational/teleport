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

import React from 'react';

import { Flex } from 'design';
import { useRefAutoFocus } from 'shared/hooks';

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
