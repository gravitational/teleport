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
import NavLeftBar from './navLeftBar';
import reactor from 'app/reactor';
import { getters } from 'app/modules/app';
import { refresh } from 'app/modules/app/actions';
import NotificationHost from './notificationHost.jsx';
import Timer from './timer.jsx';

const App = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      appStatus: getters.appStatus      
    }
  },
    
  render() {
    let {
      isInitializing      
    } = this.state.appStatus;

    if(isInitializing){
      return null;
    }
    
    return (
      <div className="grv-tlpt grv-flex grv-flex-row">      
        <Timer onTimeout={refresh} interval={4000} />
        <NotificationHost/>
        {this.props.CurrentSessionHost}
        <NavLeftBar/>
        {this.props.children}
      </div>
    );
  }
})

export default App;