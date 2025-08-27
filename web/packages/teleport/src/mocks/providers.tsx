/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { LocationDescriptor } from 'history';
import { PropsWithChildren } from 'react';
import { MemoryRouter } from 'react-router';

import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';
import { ToastNotificationProvider } from 'shared/components/ToastNotification';

import { getOSSFeatures } from 'teleport/features';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { ContextProvider } from 'teleport/index';
import { createTeleportContext } from 'teleport/mocks/contexts';
import TeleportContext from 'teleport/teleportContext';

export const TeleportProviderBasic: React.FC<
  PropsWithChildren<{
    initialEntries?: LocationDescriptor<unknown>[];
    teleportCtx?: TeleportContext;
  }>
> = ({ children, initialEntries, teleportCtx }) => {
  const ctx = teleportCtx || createTeleportContext();

  return (
    <MemoryRouter initialEntries={initialEntries}>
      <ToastNotificationProvider>
        <InfoGuidePanelProvider>
          <ContextProvider ctx={ctx}>
            <FeaturesContextProvider value={getOSSFeatures()}>
              {children}
            </FeaturesContextProvider>
          </ContextProvider>
        </InfoGuidePanelProvider>
      </ToastNotificationProvider>
    </MemoryRouter>
  );
};
