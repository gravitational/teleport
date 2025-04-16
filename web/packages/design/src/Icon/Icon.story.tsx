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

import { useEffect, useRef } from 'react';
import styled from 'styled-components';

import { Flex } from 'design';
import { blink } from 'design/keyframes';

import { Broadcast } from './Icons/Broadcast';

export default {
  // Nest stories under Icon/Icon, so that Icon/Icons which lists all icons is the first story.
  title: 'Design/Icon/Icon',
};

export const WithRef = () => {
  const nodeRef = useRef<HTMLElement>(null);

  useEffect(() => {
    nodeRef.current?.scrollIntoView({ block: 'center' });
  }, []);

  return (
    <Flex flexDirection="column" height="200vh" justifyContent="center">
      <div>
        <StyledBroadcast ref={nodeRef} />
        <p>On the first render, the view should be scrolled to the icon.</p>
      </div>
    </Flex>
  );
};

const StyledBroadcast = styled(Broadcast)`
  animation: ${blink} 1s ease-in-out infinite;
`;
