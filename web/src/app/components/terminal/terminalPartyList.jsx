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
import ReactCSSTransitionGroup from 'react-addons-css-transition-group';
import { connect } from 'nuclear-js-react-addons';
import sessionGetters from 'app/flux/sessions/getters';
import {UserIcon} from './../icons.jsx';

const PartyList = props => {
  let parties = props.parties || [];
  let userIcons = parties.map((item, index)=>(
    <div key={index} className="animated m-t">
      <UserIcon colorIndex={index}
        isDark={true}
        name={item.user} />
    </div>
  ));

  return (
    <ReactCSSTransitionGroup className="m-t" component='div'
      transitionEnterTimeout={500}
      transitionLeaveTimeout={500}
      transitionName={{
        enter: "fadeIn",
        leave: "fadeOut"
      }}>
      <hr className="grv-divider m-t" />
      {userIcons}
    </ReactCSSTransitionGroup>
  )
}

function mapStateToProps(props) {
  return {
    parties: sessionGetters.activePartiesById(props.sid)
  }
}

export default connect(mapStateToProps)(PartyList)