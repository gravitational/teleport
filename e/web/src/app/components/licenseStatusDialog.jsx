import React, { PropTypes } from 'react';
import Button from './common/button';
import classnames from 'classnames';

import {
  GrvDialogHeader,
  GrvDialogFooter,
  GrvDialog } from './common/dialog';

class LicenseStatusDialog extends React.Component {

  static propTypes = {  
    status: PropTypes.object.isRequired,
    onOk: PropTypes.func.isRequired
  }

  render(){
    const { onOk, status } = this.props;    
    const bodyText = status.html || status.text;

    const classname = classnames('grv-dialog-no-body grv-dialog-sm grv-dialog-license-status',
    {
      '--info': status.isInfo(),
      '--error': status.isError(),
      '--warning': status.isWarning()
    })

    const iconClass = classnames('fa fa-2x grv-dialog-license-status-icon', {
      'fa-exclamation-triangle': status.isError() || status.isWarning(),
      'fa-info-circle': status.isInfo()
    })

    return (
      <GrvDialog className={classname} >
        <GrvDialogHeader>
          <div className="grv-dialog-confirm-header">
            <div className="m-t-xs m-l-xs m-r-md">
              <i className={iconClass}/>
            </div>
            <div>
              <div dangerouslySetInnerHTML={{ __html: bodyText }} />
            </div>
          </div>
        </GrvDialogHeader>
        <GrvDialogFooter>
          <Button onClick={onOk} className="grv-dialog-license-status-button">
            Disregard and continue
          </Button>        
        </GrvDialogFooter>
      </GrvDialog>
    )
  }
}

export default LicenseStatusDialog;