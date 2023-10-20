/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useCallback, useEffect, useState } from 'react';
import styled from 'styled-components';
import { Indicator } from 'design';

import { useAppContext } from './appContextProvider';
import { initUi } from './initUi';
import ModalsHost from './ModalsHost';
import { LayoutManager } from './LayoutManager';

export const AppInitializer = () => {
  const ctx = useAppContext();
  const [isUiReady, setIsUiReady] = useState(false);

  const initializeApp = useCallback(async () => {
    try {
      await ctx.init();
      await initUi(ctx);
      setIsUiReady(true);
    } catch (error) {
      setIsUiReady(true);
      ctx.notificationsService.notifyError(error?.message);
    }
  }, [ctx]);

  useEffect(() => {
    initializeApp();
  }, [initializeApp]);

  return (
    <>
      <LayoutManager />
      {!isUiReady && (
        <Centered>
          <Indicator delay="short" />
        </Centered>
      )}
      <ModalsHost />
    </>
  );
};

const Centered = styled.div`
  display: flex;
  position: absolute;
  width: 100%;
  height: 100%;
  justify-content: center;
  align-items: center;
  z-index: 2; // renders the overlay above ConnectionsIconStatusIndicator
  background: ${props => props.theme.colors.levels.sunken};
`;
