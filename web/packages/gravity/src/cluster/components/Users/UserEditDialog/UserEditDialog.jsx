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
import {
  Box,
  Text,
  ButtonPrimary,
  ButtonSecondary,
} from 'design';
import * as Alerts from 'design/Alert';
import { useAttempt, withState } from 'shared/hooks';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/DialogConfirmation';
import * as actions from 'gravity/cluster/flux/users/actions';
import Validation from 'shared/components/Validation';
import FieldSelect from 'shared/components/FieldSelect';

export function UserEditDialog(props) {
  const { roles, user, onClose, attempt, attemptActions, onSave } = props;
  const { isFailed, isProcessing } = attempt;
  const { userId } = user;

  const selectOptions = roles.map(r => ({
    value: r,
    label: r,
  }));

  const [selectedRoles, setRoles] = React.useState(() =>
    user.roles.map(r => ({
      value: r,
      label: r,
    }))
  );

  const onClickSave = validator => {
    if (!validator.validate()) {
      return;
    }

    const userRoles = selectedRoles.map(r => r.value);
    attemptActions
      .do(() => onSave(userId, userRoles))
      .then(() => {
        onClose();
      });
  };

  function onChangeRoles(values) {
    setRoles(values);
  }

  return (
    <Validation>
      {({ validator }) => (
        <Dialog
          disableEscapeKeyDown={false}
          onClose={onClose}
          open={true}
          dialogCss={dialogCss}
        >
          <Box width="450px">
            <DialogHeader>
              <DialogTitle>Edit User Role</DialogTitle>
            </DialogHeader>
            <DialogContent>
              {isFailed && <Alerts.Danger>{attempt.message}</Alerts.Danger>}
              <Text mb="3" typography="paragraph" color="primary.contrastText">
                User: {userId}
              </Text>
              <FieldSelect
                maxMenuHeight="200"
                placeholder="Click to select a role"
                isSearchable
                isMulti
                isSimpleValue
                clearable={false}
                rule={isRequired}
                label="Assign a role"
                value={selectedRoles}
                options={selectOptions}
                onChange={onChangeRoles}
              />
            </DialogContent>
            <DialogFooter>
              <ButtonPrimary
                mr="3"
                disabled={isProcessing}
                onClick={() => onClickSave(validator)}
              >
                SAVE Changes
              </ButtonPrimary>
              <ButtonSecondary disabled={isProcessing} onClick={onClose}>
                Cancel
              </ButtonSecondary>
            </DialogFooter>
          </Box>
        </Dialog>
      )}
    </Validation>
  );
}

UserEditDialog.propTypes = {
  user: PropTypes.object.isRequired,
  onClose: PropTypes.func.isRequired,
};

const isRequired = values => () => {
  if (values.length === 0) {
    return {
      valid: false,
      message: 'This field is required',
    };
  }

  return {
    valid: true,
  };
};

function mapState() {
  const [attempt, attemptActions] = useAttempt();
  return {
    onSave: actions.saveUser,
    attempt,
    attemptActions,
  };
}

const dialogCss = () => `
  overflow-y: visible;
`;

export default withState(mapState)(UserEditDialog);
