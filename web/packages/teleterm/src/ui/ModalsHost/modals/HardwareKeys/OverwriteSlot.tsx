/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { ButtonPrimary, ButtonSecondary, Flex, P2 } from 'design';
import DialogConfirmation, {
  DialogContent,
  DialogFooter,
} from 'design/DialogConfirmation';
import { ConfirmHardwareKeySlotOverwriteRequest } from 'gen-proto-ts/teleport/lib/teleterm/v1/tshd_events_service_pb';

import { CommonHeader } from './CommonHeader';

export function OverwriteSlot(props: {
  req: ConfirmHardwareKeySlotOverwriteRequest;
  onCancel(): void;
  onConfirm(): void;
  hidden?: boolean;
}) {
  return (
    <DialogConfirmation
      open={!props.hidden}
      keepInDOMAfterClose
      onClose={props.onCancel}
      dialogCss={() => ({
        maxWidth: '450px',
        width: '100%',
      })}
    >
      <form
        onSubmit={e => {
          e.preventDefault();
          props.onConfirm();
        }}
      >
        <CommonHeader
          onCancel={props.onCancel}
          rootClusterUri={props.req.rootClusterUri}
        />

        <DialogContent mb={4}>
          <P2 color="text.slightlyMuted">{props.req.message}</P2>
        </DialogContent>

        <DialogFooter>
          <Flex gap={3}>
            <ButtonPrimary type="submit">Yes</ButtonPrimary>
            <ButtonSecondary type="button" onClick={props.onCancel}>
              No
            </ButtonSecondary>
          </Flex>
        </DialogFooter>
      </form>
    </DialogConfirmation>
  );
}
