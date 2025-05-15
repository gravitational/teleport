/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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
import { useEffect, useMemo } from 'react';

import { parseDeepLink } from 'teleterm/deepLinks';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useLogger } from 'teleterm/ui/hooks/useLogger';
import { useVnetContext } from 'teleterm/ui/Vnet';

import * as launchDeepLinkModule from './launchDeepLink';

/**
 * DeepLinks subscribes to deep link launch events coming from the main process and fires relevant
 * business logic handlers.
 *
 * It's defined as a separate component in the tree so that updates to contexts used by DeepLinks
 * don't cause re-renders of the whole app.
 */
export const DeepLinks = ({
  launchDeepLink = launchDeepLinkModule.launchDeepLink,
}: {
  launchDeepLink?: typeof launchDeepLinkModule.launchDeepLink;
}) => {
  const appCtx = useAppContext();
  const logger = useLogger('DeepLinks');
  const vnet = useVnetContext();
  const auxCtx = useMemo(
    () => ({
      vnet: { isSupported: vnet.isSupported },
    }),
    [vnet.isSupported]
  );

  useEffect(() => {
    const { cleanup } = appCtx.mainProcessClient.subscribeToDeepLinkLaunch(
      result => {
        launchDeepLink(appCtx, auxCtx, result).catch(error => {
          logger.error('Error when launching a deep link', error);
        });
      }
    );

    if (process.env.NODE_ENV === 'development') {
      window['deepLinkLaunch'] = (url: string) => {
        const result = parseDeepLink(url);
        launchDeepLink(appCtx, auxCtx, result).catch(error => {
          logger.error('Error when launching a deep link', error);
        });
      };
    }

    return cleanup;
  }, [appCtx, auxCtx, logger, launchDeepLink]);

  return null;
};
