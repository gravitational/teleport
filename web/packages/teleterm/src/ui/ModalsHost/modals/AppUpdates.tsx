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

import { ButtonIcon, H2 } from 'design';
import DialogConfirmation, {
  DialogContent,
  DialogHeader,
} from 'design/DialogConfirmation';
import { Cross } from 'design/Icon';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { DetailsView, useAppUpdaterContext } from 'teleterm/ui/AppUpdater';

export function AppUpdates(props: { hidden?: boolean; onClose(): void }) {
  const appContext = useAppContext();
  const { updateEvent } = useAppUpdaterContext();

  const {
    checkForAppUpdates,
    downloadAppUpdate,
    cancelAppUpdateDownload,
    quitAndInstallAppUpdate,
    changeAppUpdatesManagingCluster,
  } = appContext.mainProcessClient;

  useEffect(() => {
    void checkForAppUpdates();
  }, [checkForAppUpdates]);

  const platform = useMemo(() => {
    return appContext.mainProcessClient.getRuntimeSettings().platform;
  }, [appContext.mainProcessClient]);

  return (
    <DialogConfirmation
      open={!props.hidden}
      keepInDOMAfterClose
      onClose={props.onClose}
      dialogCss={() => ({
        maxWidth: '420px',
        width: '100%',
      })}
    >
      <DialogHeader justifyContent="space-between" mb={3} alignItems="baseline">
        <H2>App Updates</H2>
        <ButtonIcon
          type="button"
          onClick={props.onClose}
          color="text.slightlyMuted"
        >
          <Cross size="medium" />
        </ButtonIcon>
      </DialogHeader>

      <DialogContent mb={0}>
        <DetailsView
          platform={platform}
          updateEvent={updateEvent}
          clusterGetter={appContext.clustersService}
          onCancelDownload={() => void cancelAppUpdateDownload()}
          onDownload={() => void downloadAppUpdate()}
          onCheckForUpdates={() => void checkForAppUpdates()}
          onInstall={() => void quitAndInstallAppUpdate()}
          changeManagingCluster={clusterUri =>
            void changeAppUpdatesManagingCluster(clusterUri)
          }
        />
      </DialogContent>
    </DialogConfirmation>
  );
}
