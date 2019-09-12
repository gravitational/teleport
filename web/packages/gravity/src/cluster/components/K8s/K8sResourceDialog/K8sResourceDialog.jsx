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
import { useAttempt } from 'shared/hooks';
import Dialog, {
  DialogFooter,
  DialogTitle,
  DialogHeader,
  DialogContent,
} from 'design/Dialog';
import { ButtonPrimary, ButtonSecondary, Text } from 'design';
import { Danger } from 'design/Alert';

export default function K8sResourceDialog(props) {
  const { readOnly = true, resource, name, namespace, onSave, onClose } = props;
  const [attempt, attemptActions] = useAttempt();
  const { content, onChange } = useResource(resource);
  const [isDirty, setDirty] = React.useState(false);
  const saveDisabled = !isDirty || attempt.isProcessing;

  function onClickSave() {
    attemptActions
      .do(() => onSave(namespace, name, JSON.parse(content)))
      .then(() => onClose());
  }

  return (
    <Dialog
      dialogCss={dialogCss}
      disableEscapeKeyDown={false}
      onClose={onClose}
      open={true}
    >
      <DialogHeader justifyContent="space-between">
        <DialogTitle typography="h3" caps={false}>
          {name}
        </DialogTitle>
        <Text as="span">NAMESPACE: {namespace}</Text>
      </DialogHeader>
      {attempt.isFailed && <Danger mb="4">{attempt.message}</Danger>}
      <DialogContent>
        <TextEditor
          readOnly={readOnly}
          onDirty={setDirty}
          onChange={onChange}
          data={[{ content, type: 'json' }]}
        />
      </DialogContent>
      <DialogFooter>
        {!readOnly && (
          <ButtonPrimary mr="3" disabled={saveDisabled} onClick={onClickSave}>
            Save Changes
          </ButtonPrimary>
        )}
        <ButtonSecondary onClick={onClose}>Close</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

K8sResourceDialog.propTypes = {
  resource: PropTypes.object.isRequired,
  onClose: PropTypes.func.isRequired,
};

const dialogCss = () => `
  height: 80%
  width: calc(100% - 20%);
  max-width: 1400px;
`;

function useResource(json) {
  const [content, setContent] = React.useState(() => {
    json = json || {};
    return JSON.stringify(json, null, 2);
  });

  const onChange = content => setContent(content);

  return { content, onChange };
}
