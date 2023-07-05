/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import PropTypes from 'prop-types';
import TextEditor from 'shared/components/TextEditor';
import Dialog, {
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogContent,
} from 'design/Dialog';

import { ButtonSecondary } from 'design';

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
        <DialogTitle typography="body1" caps={true} bold>
          {title}
        </DialogTitle>
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
