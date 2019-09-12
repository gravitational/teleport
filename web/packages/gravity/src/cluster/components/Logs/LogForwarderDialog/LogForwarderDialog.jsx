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

import React from 'react'
import PropTypes from 'prop-types';
import Dialog, { DialogTitle } from 'design/Dialog';
import { ButtonPrimary, ButtonSecondary, Flex, Text } from 'design';
import { useAttempt, withState } from 'shared/hooks';
import { useStore, ModeEnum } from './store';
import ViewMode from './ViewMode';
import EditMode from './EditMode';

export function LogForwarderDialog(props) {
  const { store } = props;
  const { curIndex, items, mode } = store.state;
  const { setCurrent } = store;

  const onEdit = () => {
    store.setEditMode()
  }

  const onCancelEdit = () => {
    store.setViewMode();
  }

  const onCreate = () => {
    store.setNewMode();
  }

  const onClose = () => {
    props.onClose && props.onClose();
  };

  const onSave = content => {
    return store.save(content);
  };

  const onDelete = () => {
    return store.delete(curIndex);
  }

  const isNew = mode === ModeEnum.NEW;
  const showEmpty = items.length === 0 && !isNew;
  const isViewMode = mode === ModeEnum.VIEW && !showEmpty;
  const isEditMode = mode === ModeEnum.EDIT || isNew;

  return (
    <Dialog open={true} dialogCss={dialogCss} onClose={onClose}>
      { showEmpty && (
        <Flex height="500px" width="700px">
          <Flex width="250px" bg="primary.light" alignItems="center" justifyContent="center">
            No Existing Log Forwarders
          </Flex>
          <Flex flex="1" p="5" flexDirection="column" >
            <DialogTitle typography="body1" bold mb="4"> Create and Manage Log Forwarders </DialogTitle>
            <Text mb="8" typography="body1" color="primary.contrastText">
              Create your first log forwarder to ship cluster logs to a remote log collector such as a rsyslog server.

            </Text>
            <ButtonPrimary mx="auto" onClick={onCreate}>
              Create a new log forwarder
            </ButtonPrimary>
            <Flex flex="1" alignItems="flex-end">
              <ButtonSecondary mx="auto" onClick={onClose}>
                Close Log Forwarder Settings
              </ButtonSecondary>
            </Flex>
          </Flex>
        </Flex>
      )}
      { isEditMode && <EditMode height="600px" width="800px"
          onSave={onSave}
          item={items[curIndex]}
          isNew={isNew}
          onCancel={onCancelEdit}
          onDelete={onDelete}
        /> }
      { isViewMode && <ViewMode height="600px" width="800px"
          key={curIndex}
          curIndex={curIndex}
          items={items}
          onSelect={setCurrent}
          onCreate={onCreate}
          onEdit={onEdit}
          onClose={onClose}
          onDelete={onDelete}
        />
      }
    </Dialog>
  );
}

LogForwarderDialog.propTypes = {
  resource: PropTypes.object,
  onClose: PropTypes.func.isRequired,
}

const dialogCss = () => `
  max-height: 600px;
  padding: 0px;
`

export default withState(props => {
  const store = useStore(props.store);
  const [ attempt, attemptActions ] = useAttempt();
  return {
    store,
    attempt,
    attemptActions
  }
})(LogForwarderDialog);