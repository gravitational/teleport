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
import styled from 'styled-components';

interface LinearProgressProps {
  transparentBackground?: boolean;
}

const LinearProgress = (props: LinearProgressProps) => {
  return (
    <div
      style={{
        position: 'absolute',
        left: '0',
        right: '0',
        bottom: '0',
      }}
    >
      <StyledProgress transparentBackground={props.transparentBackground}>
        <div className="parent-bar-2" />
      </StyledProgress>
    </div>
  );
};

const StyledProgress = styled.div`
  position: relative;
  overflow: hidden;
  display: block;
  height: 1px;
  z-index: 0;
  background-color: ${props =>
    props.transparentBackground ? 'transparent' : props.theme.colors.surface};

  .parent-bar-2 {
    position: absolute;
    left: 0;
    bottom: 0;
    top: 0;
    transition: transform 0.2s linear;
    transform-origin: left;
    background-color: #1976d2;
    animation: animation-linear-progress 2s cubic-bezier(0.165, 0.84, 0.44, 1)
      0.1s infinite;
  }

  @keyframes animation-linear-progress {
    0% {
      left: -300%;
      right: 100%;
    }

    60% {
      left: 107%;
      right: -8%;
    }

    100% {
      left: 107%;
      right: -8%;
    }
  }
`;

export default LinearProgress;
