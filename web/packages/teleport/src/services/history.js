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

import { createBrowserHistory } from 'history';
import { matchPath } from 'react-router';
import cfg from 'teleport/config';

let _inst = null;

const history = {
  original() {
    return _inst;
  },

  init(history) {
    _inst = history || createBrowserHistory();
  },

  replace(route) {
    route = this.ensureKnownRoute(route);
    _inst.replace(route);
  },

  push(route, withRefresh = false) {
    route = this.ensureKnownRoute(route);
    if (withRefresh) {
      this._pageRefresh(route);
    } else {
      _inst.push(route);
    }
  },

  goBack(number) {
    this.original().goBack(number);
  },

  goToLogin(rememberLocation = false) {
    let url = cfg.routes.login;
    if (rememberLocation) {
      let redirectUrl = _inst.createHref(_inst.location);
      redirectUrl = this.ensureKnownRoute(redirectUrl);
      redirectUrl = this.ensureBaseUrl(redirectUrl);
      url = `${url}?redirect_uri=${redirectUrl}`;
    }

    this._pageRefresh(url);
  },

  createRedirect(location /* location || string */) {
    let route = _inst.createHref(location);
    let knownRoute = this.ensureKnownRoute(route);
    return this.ensureBaseUrl(knownRoute);
  },

  getRedirectParam() {
    return getUrlParameter('redirect_uri', this.original().location.search);
  },

  ensureKnownRoute(url) {
    url = this._canPush(url) ? url : cfg.routes.app;
    return url;
  },

  ensureBaseUrl(url) {
    url = url || '';
    if (url.indexOf(cfg.baseUrl) !== 0) {
      url = withBaseUrl(url);
    }

    return url;
  },

  getRoutes() {
    return Object.getOwnPropertyNames(cfg.routes).map(p => cfg.routes[p]);
  },

  getLocation() {
    this.original().location;
  },

  _canPush(route) {
    route = route || '';
    let routes = this.getRoutes();
    if (route.indexOf(cfg.baseUrl) === 0) {
      route = route.replace(cfg.baseUrl, '');
    }

    return routes.some(match(route));
  },

  _pageRefresh(route) {
    window.location.href = this.ensureBaseUrl(route);
  },
};

const withBaseUrl = url => cfg.baseUrl + url;

const match = url => route => {
  return matchPath(url, {
    path: route,
    exact: true,
  });
};

export default history;

export function getUrlParameter(name, path) {
  const params = new URLSearchParams(path);
  const value = params.get(name);
  return value || '';
}
