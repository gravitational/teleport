import React from 'react';
import Button from 'app/components/common/button';
import * as Alerts from './../common/alerts';
import {
  GrvDialogHeader,
  GrvDialogFooter,  
  GrvDialog } from 'app/components/common/dialog';

import { connect } from 'nuclear-js-react-addons';
import getters from 'app/flux/settings/getters';
import { closeDeleteDialog } from 'app/flux/settings/actions';

const ConfigDeleteDialog = props => {    
  const { store, attempt, onContinue } = props;
  const { isProcessing, isFailed, message } = attempt;
  const resItem = store.getResourceToDelete();  

  if( !resItem ){
    return null;
  }    
        
  const id = resItem.getName();

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
                You are about to delete resource <strong>{id}</strong>.
              </small>
            </div>
          </div>          
        </div>
        { isFailed && <Alerts.Danger>{message} </Alerts.Danger> }
      </GrvDialogHeader>      
      <GrvDialogFooter>
        <Button
          className="btn-danger"
          onClick={ () => onContinue(id) }
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

function mapStateToProps() {
  return {    
    store: getters.dialogsStore,
    attempt: getters.deleteAttempt
  }
}

export default connect(mapStateToProps)(ConfigDeleteDialog);

