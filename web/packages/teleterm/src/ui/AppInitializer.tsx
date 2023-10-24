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

import { useLogger } from 'teleterm/ui/hooks/useLogger';

import { useAppContext } from './appContextProvider';
import { initUi } from './initUi';
import ModalsHost from './ModalsHost';
import { LayoutManager } from './LayoutManager';

export const AppInitializer = () => {
  const logger = useLogger('AppInitializer');

  const appContext = useAppContext();
  const [isUiReady, setIsUiReady] = useState(false);

  const initializeApp = useCallback(async () => {
    try {
      await appContext.init();
      await initUi(appContext);
      setIsUiReady(true);
      appContext.mainProcessClient.signalUserInterfaceReadiness({
        success: true,
      });
    } catch (error) {
      logger.error(error?.message);
      setIsUiReady(true);
      appContext?.notificationsService.notifyError(error?.message);
      appContext?.mainProcessClient.signalUserInterfaceReadiness({
        success: false,
      });
    }
  }, [appContext, logger]);

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
