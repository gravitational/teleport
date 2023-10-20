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

import styled from 'styled-components';

import favicon from './favicon.png';

export const BrowserContainer = styled.div`
  position: relative;
  border-radius: 5px;
  width: 100%;
  box-shadow: 0px 0px 20px 0px rgba(0, 0, 0, 0.43);
  color: white;
`;

export const BrowserTitleBarContainer = styled.div`
  background: #040b1d;
  height: 60px;
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  border-top-left-radius: 5px;
  border-top-right-radius: 5px;
`;

export const BrowserTitleBarButtons = styled.div`
  display: flex;
  position: absolute;
  top: 50%;
  left: 10px;
  transform: translate(0, -50%);
`;

export const BrowserTitleBarButton = styled.div`
  width: 12px;
  height: 12px;
  border-radius: 50%;
  margin-right: 5px;
`;

export const BrowserContentContainer = styled.div`
  background: white;
  color: black;
  height: var(--content-height, 660px);
  overflow-y: auto;
  border-bottom-left-radius: 5px;
  border-bottom-right-radius: 5px;
`;

export const BrowserCode = styled.div`
  font-size: 12px;
  font-family: Menlo, DejaVu Sans Mono, Consolas, Lucida Console, monospace;
  line-height: 20px;
  white-space: pre-wrap;
`;

export const BrowserURLContainer = styled.div`
  padding: 4px 10px;
  width: 300px;
  text-align: center;
  border: 1px solid rgba(255, 255, 255, 0.2);
  border-radius: 5px;
  display: flex;
  align-items: center;
  justify-content: center;
`;

export const BrowserURL = styled.div`
  display: flex;
  align-items: center;
`;

export const BrowserURLIcon = styled.div`
  margin-right: 5px;
  top: 2px;
  position: relative;
`;

export const BrowserTabs = styled.div`
  display: flex;
  background: #060b1d;
  height: 36px;
`;

export const BrowserTabFavicon = styled.div`
  margin-right: 10px;
  width: 16px;
  height: 16px;
  background: url(${favicon}) no-repeat;
`;

export const BrowserTabClose = styled.div`
  margin-left: 10px;
  svg {
    position: relative;
    top: 2px;
    transform: rotate(45deg);
  }
`;

export const BrowserTab = styled.div<{ active: boolean }>`
  display: flex;
  align-items: center;
  padding: 0 10px;
  background: ${p => (p.active ? 'rgba(255, 255, 255, 0.1)' : 'none')};
  border-top-left-radius: ${p => (p.active ? '7px' : '0')};
  border-top-right-radius: ${p => (p.active ? '7px' : '0')};
  margin-right: 10px;

  ${BrowserTabClose} {
    display: ${p => (p.active ? 'block' : 'none')};
  }
`;
export const BrowserTabTitle = styled.div`
  font-size: 13px;
`;
