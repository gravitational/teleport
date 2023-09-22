/*
Copyright 2023 Gravitational, Inc.

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

import styled, { css } from 'styled-components';

export enum NavigationItemSize {
  Small,
  Large,
  Indented,
}

export const Icon = styled.div`
  flex: 0 0 24px;
  margin-right: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
`;

export const SmallIcon = styled(Icon)`
  flex: 0 0 14px;
  margin-right: 10px;
`;

interface LinkContentProps {
  size: NavigationItemSize;
}

const padding = {
  [NavigationItemSize.Small]: '7px 0px 7px 30px',
  [NavigationItemSize.Indented]: '7px 30px 7px 67px',
  [NavigationItemSize.Large]: '16px 30px',
};

export const LinkContent = styled.div<LinkContentProps>`
  display: flex;
  padding: ${p => padding[p.size]};
  align-items: center;
  width: 100%;
  opacity: 0.7;
  transition: opacity 0.15s ease-in;
`;

export const commonNavigationItemStyles = css`
  display: flex;
  position: relative;
  color: ${props => props.theme.colors.text.main};
  text-decoration: none;
  user-select: none;
  font-size: 14px;
  font-weight: 300;
  border-left: 4px solid transparent;
  transition: transform 0.3s cubic-bezier(0.19, 1, 0.22, 1);
  will-change: transform;

  &:hover {
    background: ${props => props.theme.colors.spotBackground[0]};
  }
`;
