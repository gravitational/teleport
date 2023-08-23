/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useEffect, useState } from 'react';
import styled, { keyframes } from 'styled-components';

import { SVGIconProps } from 'design/SVGIcon/common';

const Container = styled.div`
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 10;
  pointer-events: none;
`;

const CursorContainer = styled.div`
  position: absolute;
  transition: 0.3s linear;
`;

interface CursorProps {
  top: number;
  left: number;
  click: boolean;
}

const pulse = keyframes`
  0% {
    transform: scale(0.1, 0.1);
    opacity: 0;
  }
  50% {
    opacity: 1;
  }
  100% {
    transform: scale(1.6, 1.6);
    opacity: 0;
  }
`;

const fadeIn = keyframes`
  0% {
    opacity: 0;
  }
  
  20% {
    opacity: 1;
  }
  
  80% {
    opacity: 1;
  }
  
  100% { 
    opacity: 0;
  }
`;

export const Pulse = styled.div`
  background: #abc6e4;
  opacity: 0.5;
  border-radius: 50%;
  height: 14px;
  width: 14px;
  position: absolute;
  left: 8px;
  top: 3px;
  z-index: 2;
  animation: ${fadeIn} 2s ease-in forwards;

  &:after {
    content: '';
    border-radius: 50%;
    height: 40px;
    width: 40px;
    opacity: 0;
    box-shadow: 0 0 1px 2px #abc6e4;
    position: absolute;
    margin: -13px 0 0 -13px;
    animation: ${pulse} 2s ease-out infinite;
  }
`;

export function Cursor(props: CursorProps) {
  const [showPulse, setShowPulse] = useState(false);

  useEffect(() => {
    if (!props.click) {
      return;
    }

    const id = window.setTimeout(() => setShowPulse(true), 1000);
    const id2 = window.setTimeout(() => setShowPulse(false), 3000);

    return () => {
      clearTimeout(id);
      clearTimeout(id2);
    };
  }, [props.top, props.left, props.click]);

  return (
    <Container>
      <CursorContainer style={{ top: props.top, left: props.left }}>
        <div style={{ position: 'relative', zIndex: 3 }}>
          <CursorIcon />
        </div>

        {showPulse && props.click && <Pulse />}
      </CursorContainer>
    </Container>
  );
}

function CursorIcon({ size = 40, fill = 'white' }: SVGIconProps) {
  return (
    <svg
      viewBox="0 0 48 48"
      xmlns="http://www.w3.org/2000/svg"
      width={size}
      height={size}
      fill={fill}
    >
      <path d="M27.8,39.7c-0.1,0-0.2,0-0.4-0.1c-0.2-0.1-0.4-0.3-0.6-0.5l-3.7-8.6l-4.5,4.2C18.5,34.9,18.3,35,18,35c-0.1,0-0.3,0-0.4-0.1C17.3,34.8,17,34.4,17,34l0-22c0-0.4,0.2-0.8,0.6-0.9C17.7,11,17.9,11,18,11c0.2,0,0.5,0.1,0.7,0.3l16,15c0.3,0.3,0.4,0.7,0.3,1.1c-0.1,0.4-0.5,0.6-0.9,0.7l-6.3,0.6l3.9,8.5c0.1,0.2,0.1,0.5,0,0.8c-0.1,0.2-0.3,0.5-0.5,0.6l-2.9,1.3C28.1,39.7,27.9,39.7,27.8,39.7z" />
      <path
        fill="#212121"
        d="M18,12l16,15l-7.7,0.7l4.5,9.8l-2.9,1.3l-4.3-9.9L18,34L18,12 M18,10c-0.3,0-0.5,0.1-0.8,0.2c-0.7,0.3-1.2,1-1.2,1.8l0,22c0,0.8,0.5,1.5,1.2,1.8C17.5,36,17.8,36,18,36c0.5,0,1-0.2,1.4-0.5l3.4-3.2l3.1,7.3c0.2,0.5,0.6,0.9,1.1,1.1c0.2,0.1,0.5,0.1,0.7,0.1c0.3,0,0.5-0.1,0.8-0.2l2.9-1.3c0.5-0.2,0.9-0.6,1.1-1.1c0.2-0.5,0.2-1.1,0-1.5l-3.3-7.2l4.9-0.4c0.8-0.1,1.5-0.6,1.7-1.3c0.3-0.7,0.1-1.6-0.5-2.1l-16-15C19,10.2,18.5,10,18,10L18,10z"
      />
    </svg>
  );
}
