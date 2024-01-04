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
