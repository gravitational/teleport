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

import { ButtonSecondary, Text } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';

import TextSelectCopy from 'teleport/components/TextSelectCopy';

export default function ShareSession({ closeShareSession }: Props) {
  return (
    <Dialog
      dialogCss={() => ({ maxWidth: '500px', width: '100%' })}
      disableEscapeKeyDown={false}
      onClose={closeShareSession}
      open={true}
    >
      <DialogHeader>
        <DialogTitle>Share Session</DialogTitle>
      </DialogHeader>
      <DialogContent>
        <Text mb={2} mt={1}>
          Share this URL with the person you want to share your session with.
          This person must have access to this server to be able to join your
          session.
        </Text>
        <TextSelectCopy text={window.location.href} bash={false} />
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary onClick={closeShareSession}>Close</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

type Props = {
  closeShareSession: () => void;
};
