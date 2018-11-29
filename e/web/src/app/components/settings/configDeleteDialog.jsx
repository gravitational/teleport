import React, { PropTypes } from 'react';
import * as Alerts from 'telebase-app/components/alerts';
import { ResourceEnum } from '../../services/enums'
import Button from './../common/button';

import {
  GrvDialogHeader,
  GrvDialogFooter,
  GrvDialog } from '../common/dialog';

import { connect } from 'nuclear-js-react-addons';
import getters from '../../flux/settingsDialogs/getters';
import { closeDeleteDialog } from '../../flux/settingsDialogs/actions';

const getResourceKind = kind => {
  if(kind === ResourceEnum.OIDC || kind === ResourceEnum.SAML){
    return 'auth.connector'
  }

  if(kind === ResourceEnum.ROLE){
    return 'role'
  }

  if(kind === ResourceEnum.TRUSTED_CLUSTER){
    return 'trusted cluster'
  }

  return 'resource';
}

const ConfigDeleteDialog = props => {
  const { store, attempt, onContinue } = props;
  const { isProcessing, isFailed, message } = attempt;
  const resItem = store.getResourceToDelete();

  if( !resItem ){
    return null;
  }

  const name = resItem.getName();
  const kind = getResourceKind(resItem.getKind());
  const messagePrefix = `You are about to delete ${kind} `;

  return (
    <GrvDialog title="" className="grv-dialog-no-body grv-dialog-sm grv-dialog-confirm grv-settings-dlg-delete">
      <GrvDialogHeader>
        <div className="grv-dialog-confirm-header">
          <div className="m-t-xs m-l-xs m-r-md">
            <i className="fa fa-exclamation-triangle fa-2x text-danger" aria-hidden="true"></i>
          </div>
          <div>
            <h3 className="m-b-xs">Are you sure?</h3>
            <div>
              <small>
                {messagePrefix} <strong>{name}</strong>.
              </small>
            </div>
          </div>
        </div>
        { isFailed && <Alerts.Danger>{message} </Alerts.Danger> }
      </GrvDialogHeader>
      <GrvDialogFooter>
        <Button
          className="btn-danger"
          onClick={ () => onContinue(resItem) }
          isProcessing={isProcessing}
          isDisabled={isProcessing}>
          Delete
        </Button>
        <Button
          isPrimary={false}
          className="btn-white"
          isDisabled={isProcessing}
          onClick={closeDeleteDialog}>
          Cancel
        </Button>
      </GrvDialogFooter>
    </GrvDialog>
  );
}

ConfigDeleteDialog.propTypes = {
  store: PropTypes.object.isRequired,
  attempt: PropTypes.object.isRequired
};


function mapStateToProps() {
  return {
    store: getters.dialogsStore,
    attempt: getters.deleteAttempt
  }
}

export default connect(mapStateToProps)(ConfigDeleteDialog);

