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

import React, { useCallback, useEffect, useState } from 'react';
import styled from 'styled-components';
import { Indicator } from 'design';

import { useLogger } from 'teleterm/ui/hooks/useLogger';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import ModalsHost from 'teleterm/ui/ModalsHost';
import { LayoutManager } from 'teleterm/ui/LayoutManager';

import { showStartupModalsAndNotifications } from './showStartupModalsAndNotifications';

export const AppInitializer = () => {
  const logger = useLogger('AppInitializer');

  const appContext = useAppContext();
  const [shouldShowUi, setShouldShowUi] = useState(false);

  const initializeApp = useCallback(async () => {
    try {
      await appContext.pullInitialState();
      setShouldShowUi(true);
      await showStartupModalsAndNotifications(appContext);
      appContext.mainProcessClient.signalUserInterfaceReadiness({
        success: true,
      });
    } catch (error) {
      logger.error(error?.message);

      setShouldShowUi(true);
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
      {!shouldShowUi && (
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
