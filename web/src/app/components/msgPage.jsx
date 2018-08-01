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
import { withDocTitle } from './documentTitle';

export const MSG_INFO_LOGIN_SUCCESS = 'Login was successful, you can close this window and continue using tsh.';
export const MSG_ERROR_LOGIN_FAILED = 'Login unsuccessful. Please try again, if the problem persists, contact your system administrator.';
export const MSG_ERROR_DEFAULT = 'Internal Error';
export const MSG_ERROR_NOT_FOUND = '404 Not Found';
export const MSG_ERROR_NOT_FOUND_DETAILS = `Looks like the page you are looking for isn't here any longer.`;
export const MSG_ERROR_EXPIRED_INVITE = 'Invite code has expired.';
export const MSG_ERROR_EXPIRED_INVITE_DETAILS = `Looks like your invite code isn't valid anymore.`;
export const MSG_ERROR_ACCESS_DENIED = 'Access denied';

const ErrorPageEnum = {
  FAILED_TO_LOGIN: 'login_failed',
  EXPIRED_INVITE: 'expired_invite',
  NOT_FOUND: 'not_found',
  ACCESS_DENIED: 'access_denied'
};

const InfoPageEnum = {
  LOGIN_SUCCESS: 'login_success'
};

const InfoPage = withDocTitle("Info", ({ params }) => {
  const { type } = params;
  if (type === InfoPageEnum.LOGIN_SUCCESS) {
    return <SuccessfulLogin/>
  }

  return <InfoBox />
})

const ErrorPage = withDocTitle("Error", ({ params, location }) => {
  const { type } = params;
  const details = location.query.details;
  switch (type) {
    case ErrorPageEnum.FAILED_TO_LOGIN:
      return <LoginFailed message={details} />
    case ErrorPageEnum.EXPIRED_INVITE:
      return <ExpiredLink />
    case ErrorPageEnum.NOT_FOUND:
      return <NotFound />
    case ErrorPageEnum.ACCESS_DENIED:
      return  <AccessDenied message={details}/>
    default:
      return <Failed message={details}/>
  }
})

const Box = props => (
  <div className="grv-msg-page">
    <div className="grv-header">
      <i className={props.iconClass}></i>
    </div>
    {props.children}
  </div>
)

const InfoBox = props => (
  <Box iconClass="fa fa-smile-o" {...props}/>
)

const ErrorBox = props => (
  <Box iconClass="fa fa-frown-o" {...props} />
)

const ErrorBoxDetails = ({ message='' }) => (
  <div className="m-t text-muted">
    <small className="grv-msg-page-details-text">{message}</small>
    <p>
      <small className="contact-section">If you believe this is an issue with Teleport, please <a href="https://github.com/gravitational/teleport/issues/new">create a GitHub issue.</a></small>
    </p>
  </div>
)

const NotFound = () => (
  <ErrorBox>
    <h1>{MSG_ERROR_NOT_FOUND}</h1>
    <ErrorBoxDetails message={MSG_ERROR_NOT_FOUND_DETAILS}/>
  </ErrorBox>
)

const NotFoundPage = withDocTitle("Not Found", NotFound);

const AccessDenied = ({message}) => (
  <Box iconClass="fa fa-frown-o">
    <h1>{MSG_ERROR_ACCESS_DENIED}</h1>
    <ErrorBoxDetails message={message}/>
  </Box>
)

const Failed = ({message}) => (
  <ErrorBox>
    <h1>{MSG_ERROR_DEFAULT}</h1>
    <ErrorBoxDetails message={message}/>
  </ErrorBox>
)

const ExpiredLink = () => (
  <ErrorBox>
    <h1>{MSG_ERROR_EXPIRED_INVITE}</h1>
    <ErrorBoxDetails message={MSG_ERROR_EXPIRED_INVITE_DETAILS}/>
  </ErrorBox>
)

const LoginFailed = ({ message }) => (
  <ErrorBox>
    <h1>{MSG_ERROR_LOGIN_FAILED}</h1>
    <ErrorBoxDetails message={message}/>
  </ErrorBox>
)

const SuccessfulLogin = () => (
  <InfoBox>
    <h1>{MSG_INFO_LOGIN_SUCCESS}</h1>
  </InfoBox>
)

export {
  ErrorPage,
  InfoPage,
  NotFoundPage,
  NotFound,
  Failed,
  AccessDenied,
  ExpiredLink
};
