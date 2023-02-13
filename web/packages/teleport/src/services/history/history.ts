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

import { matchPath } from 'react-router';

import cfg from 'teleport/config';

export function redirectToLogin(remember = false) {
  let url = cfg.routes.login;

  if (remember) {
    let { search, pathname } = window.location;
    if (!pathExists(pathname)) {
      pathname = cfg.routes.root;
    }

    const knownRedirect = ensureBaseUrl(pathname);
    const query = search ? encodeURIComponent(search) : '';

    url = `${url}?redirect_uri=${knownRedirect}${query}`;
  }

  loadPageWithRefresh(url);
}

export function ensureBaseUrl(url: string) {
  // create a URL object with url arg and optional `base` second arg set to cfg.baseUrl
  let urlWithBase = new URL(url || '', cfg.baseUrl);

  // if an attacker tries to pass a url such as teleport.example.com.bad.io
  // the cfg.baseUrl argument will be overridden. If it hasn't been overridden we can
  // assume that the passed url is either relative, or matches our cfg.baseUrl
  if (urlWithBase.origin !== cfg.baseUrl) {
    // create a new url with our base if the base doesn't match
    urlWithBase = new URL(urlWithBase.pathname, cfg.baseUrl);
  }

  return urlWithBase.toString();
}

export function loadPageWithRefresh(url: string) {
  window.location.assign(ensureBaseUrl(url));
}

export function getUrlParameter(name = '', path = '') {
  const params = new URLSearchParams(path);

  return params.get(name) || '';
}

export function getRedirectParam() {
  return getUrlParameter('redirect_uri', window.location.search);
}

export function pathExists(route: string) {
  const knownRoutes = getRoutes();
  const { pathname } = new URL(ensureBaseUrl(route));
  const match = (known: string) =>
    // only match against pathname
    matchPath(
      {
        path: known,
        end: true,
      },
      pathname
    );

  return knownRoutes.some(match);
}

function getRoutes() {
  return Object.getOwnPropertyNames(cfg.routes).map(p => cfg.routes[p]);
}
