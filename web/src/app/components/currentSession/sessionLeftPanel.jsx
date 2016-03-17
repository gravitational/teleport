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
var {actions} = require('app/modules/activeTerminal/');
var colors = ['#104137', '#1c84c6', '#23c6c8', '#f8ac59', '#ED5565', '#c2c2c2'];
var ReactCSSTransitionGroup = require('react-addons-css-transition-group');

const UserIcon = ({name})=>{
  let color = colors[0];
  let style = {
    'backgroundColor': color,
    'borderColor': color
  };

  return (
    <li title={name} className="animated">
      <button style={style} className="btn btn-primary btn-circle text-uppercase">
        <strong>{name[0]}</strong>
      </button>
    </li>
  )
};

const SessionLeftPanel = ({parties}) => {
  parties = parties || [];
  let userIcons = parties.map((item, index)=>(
    <UserIcon key={index} colorIndex={index} name={item.user}/>
  ));

  return (
    <div className="grv-terminal-participans">
      <ul className="nav">
        <li title="Close">
          <button onClick={actions.close} className="btn btn-danger btn-circle" type="button">
            <i className="fa fa-times"></i>
          </button>
        </li>
      </ul>
      <hr className="grv-divider"/>
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
