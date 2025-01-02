/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { useEffect, useRef, useState } from 'react';
import { CSSTransition } from 'react-transition-group';
import styled from 'styled-components';

import { Broadcast } from './Icons/Broadcast';

export default {
  // Nest stories under Icon/Icon, so that Icon/Icons which lists all icons is the first story.
  title: 'Design/Icon/Icon',
};

export const WithRef = () => {
  const [isShown, setIsShown] = useState(true);
  const nodeRef = useRef(null);

  useEffect(() => {
    const interval = setInterval(() => {
      setIsShown(value => !value);
    }, timeoutMs * 3);

    return () => {
      clearInterval(interval);
    };
  }, []);

  return (
    // This can be done with pure CSS, it's implemented like this just to show how Icon interacts
    // with ref.
    <CSSTransition
      nodeRef={nodeRef}
      in={isShown}
      timeout={timeoutMs}
      classNames="node"
    >
      <StyledBroadcast ref={nodeRef} />
    </CSSTransition>
  );
};

const timeoutMs = 200;

const StyledBroadcast = styled(Broadcast)`
  &.node-enter {
    opacity: 0;
  }

  &.node-enter-active,
  &.node-enter-done {
    opacity: 1;
    transition: opacity ${timeoutMs}ms;
  }

  &.node-exit {
    opacity: 1;
  }

  &.node-exit-active,
  &.node-exit-done {
    opacity: 0;
    transition: opacity ${timeoutMs}ms;
  }
`;
