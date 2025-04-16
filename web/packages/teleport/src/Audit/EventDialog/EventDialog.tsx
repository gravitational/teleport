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

import PropTypes from 'prop-types';

import { ButtonSecondary } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import TextEditor from 'shared/components/TextEditor';

import { Event } from 'teleport/services/audit';

type EventDialogProps = {
  event: Event;
  onClose: () => void;
};

function EventDialog(props: EventDialogProps) {
  const { event, onClose } = props;
  const json = JSON.stringify(event.raw, null, 2);
  const title = event.codeDesc || 'Event Details';
  return (
    <Dialog
      dialogCss={dialogCss}
      disableEscapeKeyDown={false}
      onClose={onClose}
      open={true}
    >
      <DialogHeader>
        <DialogTitle>{title}</DialogTitle>
      </DialogHeader>
      <DialogContent>
        <TextEditor readOnly={true} data={[{ content: json, type: 'json' }]} />
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary onClick={onClose}>Close</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

EventDialog.propTypes = {
  event: PropTypes.object.isRequired,
  onClose: PropTypes.func.isRequired,
};

const dialogCss = () => `
  min-height: 100px;
  max-height: 80%;
  height: 100%;
  min-width: 100px;
  max-width: 80%;
  width: 100%;
`;

export default EventDialog;
