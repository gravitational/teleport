import React from 'react';
import Button from 'app/components/common/button';
import {
  GrvDialogHeader,
  GrvDialogFooter,
  GrvDialog } from 'app/components/common/dialog';

var DeleteRoleDialog = React.createClass({

  render() {
    let { roleName, attemp, onContinue, onCancel} = this.props;

    if( !roleName ){
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
                  You are about to delete <strong>{roleName}</strong> role.
                </small>
              </div>
            </div>
          </div>
        </GrvDialogHeader>
        <GrvDialogFooter>
          <Button
            className="btn-danger"
            onClick={ () => onContinue(roleName) }
            isProcessing={isProcessing}>
            Delete
          </Button>
          <Button
            isPrimary={false}
            className="btn btn-white"
            isDisabled={isProcessing}
            onClick={onCancel}>
            Close
          </Button>
        </GrvDialogFooter>
      </GrvDialog>
    );
  }
});

export default DeleteRoleDialog;
