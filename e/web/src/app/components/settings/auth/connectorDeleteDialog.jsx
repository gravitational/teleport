import React from 'react';
import Button from 'app/components/common/button';
import {
  GrvDialogHeader,
  GrvDialogFooter,
  GrvDialog } from 'app/components/common/dialog';

var DeleteConnectorDialog = React.createClass({

  render() {
    let { connectorId, attemp, onContinue, onCancel} = this.props;

    if( !connectorId ){
      return null;
    }    
        
    let { isProcessing } = attemp;

    return (
      <GrvDialog title="" className="grv-dialog-no-body grv-dialog-sm grv-dialog-confirm">
        <GrvDialogHeader>
          <div className="grv-dialog-confirm-header">
            <div className="m-t-xs m-l-xs m-r-md">
              <i className="fa fa-exclamation-triangle fa-2x text-danger" aria-hidden="true"></i>
            </div>
            <div>
              <h3 className="m-b-xs">Are you sure?</h3>
              <div>
                <small>
                  You are about to delete <strong>{connectorId}</strong> auth provider.
                </small>
              </div>
            </div>
          </div>
        </GrvDialogHeader>
        <GrvDialogFooter>
          <Button
            className="btn-danger"
            onClick={ () => onContinue(connectorId) }
            isProcessing={isProcessing}
            isDisabled={isProcessing}>
            Delete
          </Button>
          <Button
            isPrimary={false}
            className="btn-white"
            isDisabled={isProcessing}
            onClick={onCancel}>
            Close
          </Button>
        </GrvDialogFooter>
      </GrvDialog>
    );
  }
});

export default DeleteConnectorDialog;
