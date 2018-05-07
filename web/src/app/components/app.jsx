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

import React, { Component } from 'react';
import classnames from 'classnames';
import { connect } from 'nuclear-js-react-addons';
import appGetters from 'app/flux/app/getters';
import { platform } from 'app/services/browser';
import { refresh } from 'app/flux/app/actions';
import NavLeftBar from './navLeftBar';
import AjaxPoller from './dataProvider.jsx';
import { Failed } from './msgPage.jsx';
import Indicator from './indicator.jsx';

class App extends Component {
      
  render() {    
    const { router, initAttempt } = this.props;
    const { isProcessing, isSuccess, isFailed, message } = initAttempt;
        
    if (isProcessing) {      
      return <Indicator type={'bounce'} />
    }

    
    if (isFailed) {
      return <Failed message={message}/>
    }
    
    const className = classnames('grv-tlpt grv-flex grv-flex-row', {
      '--isLinux': platform.isLinux,
      '--isWin': platform.isWin,
      '--isMac': platform.isMac      
    })

    if (isSuccess) {
      return (
        <div className={className}>
          <AjaxPoller onFetch={refresh} time={3000}/>
          {this.props.CurrentSessionHost}
          <NavLeftBar router={router}/>
          {this.props.children}
        </div>
      );
    }

    return null;
  }
}

function mapStateToProps() {
  return {    
    initAttempt: appGetters.initAttempt      
  }
}

export default connect(mapStateToProps)(App);