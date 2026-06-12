/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { useEffect } from 'react';
import {
  Router,
  useLocation,
  useParams,
  useRouteMatch,
  withRouter,
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

  useEffect(() => {
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
