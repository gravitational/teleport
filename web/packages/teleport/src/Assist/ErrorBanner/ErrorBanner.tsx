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

import React, { PropsWithChildren, useState } from 'react';
import styled, { keyframes } from 'styled-components';

import { CloseIcon, ErrorIcon } from 'design/SVGIcon';

interface ErrorBannerProps {
  onDismiss: () => void;
}

const appear = keyframes`
  to {
    opacity: 1;
  }
`;

const disappear = keyframes`
  to {
    opacity: 0;
  }
`;

const Container = styled.div`
  position: relative;
  border-top: 2px solid ${props => props.theme.colors.error.main};
  border-bottom: 2px solid ${props => props.theme.colors.error.main};
  padding: 5px 45px 5px 10px;
  display: flex;
  align-items: center;
  color: ${props => props.theme.colors.error.active};
  opacity: ${p => (p.hiding ? 1 : 0)};
  animation: ${p => (p.hiding ? disappear : appear)} 0.5s ease-in-out forwards;

  & + & {
    border-top: none;
  }
`;

const ErrorIconContainer = styled.div`
  display: flex;
  align-items: center;

  svg {
    margin-right: 10px;
  }

  svg path {
    fill: ${props => props.theme.colors.error.active};
  }
`;

const CloseButton = styled.div`
  position: absolute;
  top: 50%;
  right: 5px;
  transform: translateY(-50%);
  cursor: pointer;
  padding: 5px;
  border-radius: 5px;
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 999;

  &:hover {
    background: ${p => p.theme.colors.spotBackground[0]};
  }

  svg path {
    stroke: ${p => p.theme.colors.text.main};
  }
`;

export function ErrorBanner(props: PropsWithChildren<ErrorBannerProps>) {
  const [hiding, setHiding] = useState(false);

  function handleClose() {
    setHiding(true);
    setTimeout(() => props.onDismiss(), 500);
  }

  return (
    <Container hiding={hiding}>
      <ErrorIconContainer>
        <ErrorIcon size={18} />
      </ErrorIconContainer>

      {props.children}

      <CloseButton onClick={handleClose}>
        <CloseIcon size={18} />
      </CloseButton>
    </Container>
  );
}
