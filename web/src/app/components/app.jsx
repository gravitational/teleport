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
import { connect } from 'nuclear-js-react-addons';
import cfg from 'app/config'
import { getters } from 'app/flux/app';
import { refresh } from 'app/flux/app/actions';
import NavLeftBar from './navLeftBar';
import NotificationHost from './notificationHost.jsx';
import Timer from './timer.jsx';
import { Failed } from './msgPage.jsx';
import Indicator from './indicator.jsx';

const menuItems = [        
  { icon: 'fa fa-share-alt', to: cfg.routes.nodes, title: 'Nodes' },
  { icon: 'fa  fa-group', to: cfg.routes.sessions, title: 'Sessions'}  
]

class App extends Component {
      
  getMenuItems() {    
    return menuItems;
  }

  render() {    
    const { isProcessing, isSuccess, isFailed, message } = this.props.initAttemp;
        
    if (isProcessing) {      
      return (
        <div>
          <Indicator type={'bounce'} />
        </div>
      )
    }

    if (isFailed) {
      return <Failed message={message}/>
    }
    
    if (isSuccess) {
      return (
        <div className="grv-tlpt grv-flex grv-flex-row">
          <Timer onTimeout={refresh} interval={4000}/>
          <NotificationHost />
          {this.props.CurrentSessionHost}
          <NavLeftBar items={this.getMenuItems()} />
          {this.props.children}
        </div>
      );
    }

    return null;
  }

}

function mapStateToProps() {
  return {    
    initAttemp: getters.initAttemp      
  }
}

const Connector = connect(mapStateToProps);

export default Connector(App);

export {
  App,
  Connector
}
