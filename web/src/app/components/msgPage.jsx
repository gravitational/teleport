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

const MSG_INFO_LOGIN_SUCCESS = 'Login was successful, you can close this window and continue using tsh.';
const MSG_ERROR_LOGIN_FAILED = 'Login unsuccessful. Please try again, if the problem persists, contact your system administrator.';
const MSG_ERROR_DEFAULT = 'Whoops, something went wrong.';

const MSG_ERROR_NOT_FOUND = 'Whoops, we cannot find that.';
const MSG_ERROR_NOT_FOUND_DETAILS = `Looks like the page you are looking for isn't here any longer.`;

const MSG_ERROR_EXPIRED_INVITE = 'Invite code has expired.';
const MSG_ERROR_EXPIRED_INVITE_DETAILS = `Looks like your invite code isn't valid anymore.`;

const MSG_ERROR_ACCESS_DENIED = 'Access denied';

const MsgType = {
  INFO: 'info',
  ERROR: 'error'
}

const ErrorTypes = {
  FAILED_TO_LOGIN: 'login_failed',
  EXPIRED_INVITE: 'expired_invite',
  NOT_FOUND: 'not_found',
  ACCESS_DENIED: 'access_denied'
};

const InfoTypes = {
  LOGIN_SUCCESS: 'login_success'
};

const MessagePage = ({params}) => {
  let {type, subType} = params;
  if (type === MsgType.ERROR) {
    return <ErrorPage type={subType}/>
  }

  if (type === MsgType.INFO) {
    return <InfoPage type={subType}/>
  }

  return null;
}

const ErrorMessage = props => (
  <div className="m-t" style={{ wordBreak: "break-all" }}>
    <small>{props.children}</small>
  </div>
)

const ErrorPage = ({ type, message='' }) => {      
  let msgBody = (
    <div>
      <h1>{MSG_ERROR_DEFAULT}</h1>
      <ErrorMessage>{message}</ErrorMessage>      
    </div>
  );
  
  if(type === ErrorTypes.FAILED_TO_LOGIN){
    msgBody = (
      <div>
        <h1>{MSG_ERROR_LOGIN_FAILED}</h1>
      </div>
    )
  }

  if(type === ErrorTypes.EXPIRED_INVITE){
    msgBody = (
      <div>
        <h1>{MSG_ERROR_EXPIRED_INVITE}</h1>        
        <ErrorMessage>{MSG_ERROR_EXPIRED_INVITE_DETAILS}</ErrorMessage>              
      </div>
    )
  }

  if( type === ErrorTypes.NOT_FOUND){
    msgBody = (
      <div>
        <h1>{MSG_ERROR_NOT_FOUND}</h1>        
        <ErrorMessage>{MSG_ERROR_NOT_FOUND_DETAILS}</ErrorMessage>              
      </div>
    );
  }

  if (type === ErrorTypes.ACCESS_DENIED) {
    msgBody = (
      <div>
        <h1>{MSG_ERROR_ACCESS_DENIED}</h1>
        <ErrorMessage>{message}</ErrorMessage>              
      </div>
    );    
  }

  return (
    <div className="grv-msg-page">
      <div className="grv-header"><i className="fa fa-frown-o"></i> </div>
      {msgBody}
      <small className="contact-section">If you believe this is an issue with Teleport, please <a href="https://github.com/gravitational/teleport/issues/new">create a GitHub issue.</a></small>
    </div>
  );
}

const InfoPage = ({type}) => {
  let msgBody = null;
  if (type === InfoTypes.LOGIN_SUCCESS) {
    msgBody = (
      <div>
        <h1>{MSG_INFO_LOGIN_SUCCESS}</h1>
      </div>
    );
  }

  return (
    <div className="grv-msg-page">
      <div className="grv-header">
        <i className="fa fa-smile-o"></i>
      </div>
      {msgBody}
    </div>
  );
}

var NotFound = () => (
  <ErrorPage type={ErrorTypes.NOT_FOUND}/>
)

var Failed = ({ message }) => (
  <ErrorPage message={message}/>
)

var AccessDenied = ({ message }) => (
  <ErrorPage type={ErrorTypes.ACCESS_DENIED} message={message}/>
)
    
export {
  ErrorPage,
  InfoPage,
  NotFound,
  Failed,
  AccessDenied,
  ErrorTypes,
  MessagePage
};
