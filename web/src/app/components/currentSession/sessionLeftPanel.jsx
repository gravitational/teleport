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

var React = require('react');
var {actions} = require('app/modules/currentSession/');
var {UserIcon} = require('./../icons.jsx');
var ReactCSSTransitionGroup = require('react-addons-css-transition-group');

const SessionLeftPanel = ({parties}) => {
  parties = parties || [];
  let userIcons = parties.map((item, index)=>(
    <li key={index} className="animated"><UserIcon colorIndex={index} isDark={true} name={item.user}/></li>
  ));

  return (
    <div className="grv-terminal-participans">
      <ul className="nav">
        <li title="Close">
          <button onClick={actions.close} className="btn btn-danger btn-circle" type="button">
            <span>✖</span>
          </button>
        </li>
      </ul>
      { userIcons.length > 0 ? <hr className="grv-divider"/> : null }
      <ReactCSSTransitionGroup className="nav" component='ul'
        transitionEnterTimeout={500}
        transitionLeaveTimeout={500}
        transitionName={{
          enter: "fadeIn",
          leave: "fadeOut"
        }}>
        {userIcons}
      </ReactCSSTransitionGroup>
    </div>
  )
};

module.exports = SessionLeftPanel;
