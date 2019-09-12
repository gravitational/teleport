/*
Copyright 2019 Gravitational, Inc.

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
import { withRouter, Router } from 'react-router';
import * as RouterDOM from 'react-router-dom';
import { DocumentTitle } from 'design';
import { NotFound }  from 'design/CardError';

const NoMatch = ({ location }) => (
  <NotFound message={location.pathname}/>
)

// Adds default not found handler
const Switch = props => (
  <RouterDOM.Switch>
    {props.children}
    <Route component={NoMatch}/>
  </RouterDOM.Switch>
)

const Route = ({component: Component, title, ...rest}) => (
  <RouterDOM.Route {...rest} render={props => {
    if(!title){
      return <Component {...props} />
    }

    return (
      <DocumentTitle title={title}>
        <Component {...props} />
      </DocumentTitle>
    )
  }}
  />
)

const NavLink = RouterDOM.NavLink;
const Redirect = RouterDOM.Redirect;

export {
  Router,
  Route,
  Switch,
  NavLink,
  Redirect,
  withRouter
}