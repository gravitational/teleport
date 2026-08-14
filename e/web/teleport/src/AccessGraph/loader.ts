import * as propTypes from 'prop-types';
import React, { ComponentType } from 'react';
import ReactDOM from 'react-dom';
import * as reactIs from 'react-is';
import * as jsxRuntime from 'react/jsx-runtime';
import * as styledComponents from 'styled-components';
import * as whatwgFetch from 'whatwg-fetch';

export const ACCESS_GRAPH_ASSETS_PREFIX = '/enterprise/accessgraph/static';
export const ACCESS_GRAPH_JS_FILE = 'access-graph-react-19.umd.js';
export const ACCESS_GRAPH_JS_URL = `${ACCESS_GRAPH_ASSETS_PREFIX}/${ACCESS_GRAPH_JS_FILE}`;
const STYLE_URL = `${ACCESS_GRAPH_ASSETS_PREFIX}/style.css`;

declare global {
  interface Window {
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

export function loadAccessGraph<T>(
  url: string,
  selector: () => ComponentType<T>
) {
  // we pass through a bunch of libraries to the access graph library so that the
  // library can use the same versions as the rest of the app, as well as reducing the
  // size of the library

  // styled components is a special case because it has a default export
  const styled: any = styledComponents.default; // use `any` type as typing this is tricky

  for (const key in styledComponents) {
    if (key === 'default') {
      continue;
    }

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

  return new Promise<{
    default: React.ComponentType<T>;
  }>((resolve, reject) => {
    const style = document.createElement('link');

    style.rel = 'stylesheet';
    style.href = STYLE_URL;

    document.head.appendChild(style);

    const script = document.createElement('script');

    script.src = `${ACCESS_GRAPH_ASSETS_PREFIX}/${url}`;

    script.onload = () => resolve({ default: selector() });
    script.onerror = reject;

    document.body.appendChild(script);
  });
}
