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
import {
  useRouteMatch,
  useParams,
  useLocation,
  withRouter,
  Router,
} from 'react-router';
import * as RouterDOM from 'react-router-dom';
import { NotFound } from 'design/CardError';

const NoMatch = () => (
  <NotFound
    alignSelf="baseline"
    message="The requested path could not be found."
  />
);

// Adds default not found handler
const Switch = props => (
  <RouterDOM.Switch>
    {props.children}
    <Route component={NoMatch} />
  </RouterDOM.Switch>
);

const Route = props => {
  const { title = '', ...rest } = props;
  const { clusterId } = useParams();

  React.useEffect(() => {
    if (title && clusterId) {
      document.title = `${clusterId} • ${title}`;
    } else if (title) {
      document.title = `${title}`;
    }
  }, [title]);

  return <RouterDOM.Route {...rest} />;
};

const NavLink = RouterDOM.NavLink;
const Redirect = RouterDOM.Redirect;

export {
  Router,
  Route,
  Switch,
  NavLink,
  Redirect,
  withRouter,
  useRouteMatch,
  useParams,
  useLocation,
};
