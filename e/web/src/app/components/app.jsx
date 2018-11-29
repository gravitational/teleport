/*
Copyright 2015 Gravitational, Inc.

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

// telebase
import App from 'telebase-app/components/app.jsx';
import appGetters from 'telebase-app/flux/app/getters';
import connect from 'telebase-app/components/connect';
import { Failed } from 'telebase-app/components/msgPage.jsx';
import Indicator from 'telebase-app/components/indicator.jsx';

// local
import LicenseStatus from './licenseStatusDialog';
import * as licenseGetters from './../flux/license/getters';

class TeleportE extends React.Component {

  state = {
    hasSeenLicenseMessage: false
  }

  onClose = () => {
    this.setState({hasSeenLicenseMessage: true});
  }

  render() {
    const props = this.props;
    const { initAttempt, licenseStatus } = props;
    const { isProcessing, isSuccess, isFailed, message } = initAttempt;

    if (isProcessing) {
      return <Indicator type={'bounce'} />
    }

    if (isFailed) {
      return <Failed message={message}/>
    }

    if(licenseStatus && !this.state.hasSeenLicenseMessage){
      return (
        <LicenseStatus status={licenseStatus} onOk={this.onClose}/>
      )
    }

    if (isSuccess) {
      return React.createElement(App, {
        ...props
      });
    }

    return null;
  }
}

function mapFluxToProps() {
  return {
    initAttempt: appGetters.initAttempt,
    licenseStatus: licenseGetters.store
  }
}

export default connect(mapFluxToProps)(TeleportE);

