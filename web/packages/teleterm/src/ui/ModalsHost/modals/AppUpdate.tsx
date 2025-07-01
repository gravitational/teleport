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

import { useEffect, useRef } from 'react';

import { ButtonIcon, H2 } from 'design';
import DialogConfirmation, {
  DialogContent,
  DialogHeader,
} from 'design/DialogConfirmation';
import { Cross } from 'design/Icon';

import { useAppUpdaterContext } from 'teleterm/ui/AppUpdater/AppUpdaterContext';
import { Details } from 'teleterm/ui/AppUpdater/Details';

export function AppUpdate(props: { hidden?: boolean; onCancel(): void }) {
  const updaterContext = useAppUpdaterContext();
  const checked = useRef(false);

  const { updateEvent, checkForAppUpdates, quitAndInstall } = updaterContext;

  useEffect(() => {
    console.log('elo3');
    if (checked.current) {
      return;
    }
    if (
      updateEvent.kind === 'update-available' ||
      updateEvent.kind === 'update-downloaded' ||
      updateEvent.kind === 'download-progress'
    ) {
      return;
    }
    checked.current = true;
    checkForAppUpdates();
  }, [props]);

  return (
    <DialogConfirmation
      open={!props.hidden}
      keepInDOMAfterClose
      onClose={props.onCancel}
      dialogCss={() => ({
        maxWidth: '420px',
        width: '100%',
      })}
    >
      <DialogHeader justifyContent="space-between" mb={1} alignItems="baseline">
        <H2 mb={3}>App Update</H2>
        <ButtonIcon
          type="button"
          onClick={props.onCancel}
          color="text.slightlyMuted"
        >
          <Cross size="medium" />
        </ButtonIcon>
      </DialogHeader>

      <DialogContent mb={0}>
        <Details
          updateEvent={updaterContext.updateEvent}
          onCheckForUpdates={() => updaterContext.checkForAppUpdates()}
          onInstall={() => updaterContext.quitAndInstall()}
          changeUpdatesSource={updaterContext.changeUpdatesSource}
        />
      </DialogContent>
    </DialogConfirmation>
  );
}
