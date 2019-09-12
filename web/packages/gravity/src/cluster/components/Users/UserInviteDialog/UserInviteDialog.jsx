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
import Validation from 'shared/components/Validation';
import { Box, ButtonPrimary, ButtonSecondary } from 'design';
import FieldInput from 'shared/components/FieldInput';
import FieldSelect from 'shared/components/FieldSelect';
import * as Alerts from 'design/Alert';
import { useAttempt, withState } from 'shared/hooks';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';
import * as actions from 'gravity/cluster/flux/users/actions';
import CmdText from 'gravity/components/CmdText';

export function UserInviteDialog(props) {
  const { roles, onClose, attempt, attemptActions, onCreateInvite } = props;
  const { isFailed, isProcessing, isSuccess } = attempt;
  // username input value
  const [username, setUsername] = React.useState('');
  // role selector input value
  const [selectedRoles, setRoles] = React.useState([]);

  function onCreate(validator) {
    if (!validator.validate()) {
      return;
    }

    const userRoles = selectedRoles.map(r => r.value);
    attemptActions.start();
    onCreateInvite(username, userRoles)
      .then(userToken => {
        attemptActions.stop(userToken.url);
      })
      .fail(err => {
        attemptActions.error(err);
      });
  }

  const selectOptions = roles.map(r => ({
    value: r,
    label: r,
  }));

  return (
    <Validation>
      {({ validator }) => (
        <Dialog
          dialogCss={dialogCss}
          disableEscapeKeyDown={false}
          onClose={onClose}
          open={true}
        >
          <Box width="500px">
            <DialogHeader>
              <DialogTitle> CREATE A USER INVITE LINK </DialogTitle>
            </DialogHeader>
            <DialogContent>
              {isFailed && <Alerts.Danger children={attempt.message} />}
              {isSuccess && <CmdText cmd={attempt.message} />}
              {!isSuccess && (
                <>
                  <FieldInput
                    label="User Name"
                    rule={isRequired}
                    autoFocus
                    autoComplete="off"
                    value={username}
                    onChange={e => setUsername(e.target.value)}
                  />
                  <FieldSelect
                    label="Assign a role"
                    rule={isRequired}
                    maxMenuHeight="200"
                    placeholder="Click to select a role"
                    isSearchable
                    isMulti
                    isSimpleValue
                    clearable={false}
                    value={selectedRoles}
                    onChange={values => setRoles(values)}
                    options={selectOptions}
                  />
                </>
              )}
            </DialogContent>
            <DialogFooter>
              {isSuccess && (
                <ButtonSecondary onClick={onClose}>Close</ButtonSecondary>
              )}
              {!isSuccess && (
                <>
                  <ButtonPrimary
                    mr="3"
                    disabled={isProcessing}
                    onClick={() => onCreate(validator)}
                  >
                    CREATE INVITE LINK
                  </ButtonPrimary>
                  <ButtonSecondary disabled={isProcessing} onClick={onClose}>
                    Cancel
                  </ButtonSecondary>
                </>
              )}
            </DialogFooter>
          </Box>
        </Dialog>
      )}
    </Validation>
  );
}

const isRequired = value => () => {
  if (!value || value.length === 0) {
    return {
      valid: false,
      message: 'This field is required',
    };
  }

  return {
    valid: true,
  };
};

const dialogCss = () => `
  overflow-y: visible;
`;

function mapState() {
  const [attempt, attemptActions] = useAttempt();
  return {
    onCreateInvite: actions.createInvite,
    attempt,
    attemptActions,
  };
}

export default withState(mapState)(UserInviteDialog);
