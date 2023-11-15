/*
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// This is the loader for the access graph component
//
// It will download the access graph library from the CDN and then render the component

import React, { lazy, Suspense } from 'react';
import ReactDOM from 'react-dom';
import * as styledComponents from 'styled-components';
import * as jsxRuntime from 'react/jsx-runtime';
import * as reactIs from 'react-is';
import * as whatwgFetch from 'whatwg-fetch';
import * as propTypes from 'prop-types';

const ASSETS_PREFIX = '/enterprise/accessgraph/static';
const STYLE_URL = `${ASSETS_PREFIX}/style.css`;
const JS_URL = `${ASSETS_PREFIX}/access-graph.umd.js`;

declare global {
  interface Window {
    AccessGraphLib: {
      AccessGraph: React.ComponentType<any>;
    };
    AccessGraph: {
      React: typeof React;
      ReactDOM: typeof ReactDOM;
      styled: typeof styledComponents;
      jsxRuntime: typeof jsxRuntime;
      reactIs: typeof reactIs;
      whatwgFetch: typeof whatwgFetch;
      propTypes: typeof propTypes;
    };
  }
}

function loadAccessGraph() {
  // we pass through a bunch of libraries to the access graph library so that the
  // library can use the same versions as the rest of the app, as well as reducing the
  // size of the library

  // styled components is a special case because it has a default export
  const styled: any = styledComponents.default; // use `any` type as typing this is tricky

  for (const key in styledComponents) {
    if (key === 'default') {
      continue;
    }

    // eslint-disable-next-line import/namespace
    styled[key] = styledComponents[key];
  }

  window.AccessGraph = {
    React,
    ReactDOM,
    styled,
    jsxRuntime,
    reactIs,
    whatwgFetch,
    propTypes,
  };

  return new Promise<{ default: React.ComponentType<any> }>(
    (resolve, reject) => {
      const style = document.createElement('link');

      style.rel = 'stylesheet';
      style.href = STYLE_URL;

      document.head.appendChild(style);

      const script = document.createElement('script');

      script.src = JS_URL;

      script.onload = () =>
        resolve({ default: window.AccessGraphLib.AccessGraph });
      script.onerror = reject;

      document.body.appendChild(script);
    }
  );
}

const Graph = lazy(loadAccessGraph);

export function AccessGraph() {
  return (
    <Suspense fallback={<div>Loading...</div>}>
      <Graph />
    </Suspense>
  );
}
