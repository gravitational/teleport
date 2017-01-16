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
import { Link } from  'react-router';
import {Cell} from  'app/components/table.jsx';
import moment from 'moment';

const DateCreatedCell = ({ rowIndex, data, ...props }) => {
  let { createdDisplayText } = data[rowIndex];  
  return (
    <Cell {...props}>
      { createdDisplayText }
    </Cell>
  )
};

const DurationCell = ({ rowIndex, data, ...props }) => {
  let { duration } = data[rowIndex];    
  let displayDate = moment.duration(duration).humanize();
  return (
    <Cell {...props}>
      { displayDate }
    </Cell>
  )
};

const SingleUserCell = ({ rowIndex, data, ...props }) => {  
  let { user } = data[rowIndex];
  return (
    <Cell {...props}>
      <span className="grv-sessions-user label label-default">{user}</span>
    </Cell>
  )
};

const UsersCell = ({ rowIndex, data, ...props }) => {
  let { parties, user } = data[rowIndex];
  let $users = <div className="grv-sessions-user">{user}</div> 

  if (parties.length > 0) {
    $users = parties.map((item, itemIndex) => {      
      return(
        <div key={itemIndex} className="grv-sessions-user">{item}</div>
      )
    })    
  }
      
  return (
    <Cell {...props}>
      <div>
        {$users}
      </div>
    </Cell>
  )
};

const SessionIdCell = ({ rowIndex, data, ...props }) => {
  let { sessionUrl, active, sid } = data[rowIndex];
  let [actionText, actionClass] = active ? ['join', 'btn-warning'] : ['play', 'btn-primary'];
  return (
    <Cell {...props}>
      <Link 
        to={sessionUrl}
        className={"btn " + actionClass + " btn-xs m-r-sm"}
        type="button">
          {actionText}
      </Link>
      <span> {sid} </span>
    </Cell>
  )
}

const NodeCell = ({ rowIndex, data, ...props }) => {
  let { nodeDisplayText } = data[rowIndex];       
  return (
    <Cell {...props}>
      {nodeDisplayText}
    </Cell>
  )
}

export {
  SessionIdCell,
  UsersCell,
  DurationCell,
  DateCreatedCell,
  SingleUserCell,
  NodeCell
};
