import * as propTypes from 'prop-types';
import React, { ComponentType } from 'react';
import ReactDOM from 'react-dom';
import * as reactIs from 'react-is';
import * as reactRouter from 'react-router';
import * as jsxRuntime from 'react/jsx-runtime';
import * as styledComponents from 'styled-components';

import { FetchError } from 'e-teleport/services/clienterror';

import type { CloudUIProps } from './Cloud';

const ASSETS_PREFIX = '/v1/enterprise/cloud/assets';

declare global {
  interface Window {
    CloudDeps: {
      React: typeof React;
      ReactDOM: typeof ReactDOM;
      styled: typeof styledComponents;
      jsxRuntime: typeof jsxRuntime;
      reactIs: typeof reactIs;
      propTypes: typeof propTypes;
      reactRouter: typeof reactRouter;
    };
    Cloud: { Cloud: ComponentType<CloudUIProps> };
  }
}

export function loadCloud(url: string) {
  return new Promise<{
    default: React.ComponentType<CloudUIProps>;
  }>((resolve, reject) => {
    // Pass dependencies to the Cloud lib.
    // styled components is a special case because it has a default export
    const styled: any = styledComponents.default; // use `any` type as typing this is tricky

    for (const key in styledComponents) {
      if (key === 'default') {
        continue;
      }

      styled[key] = styledComponents[key];
    }

    window.CloudDeps = {
      React,
      ReactDOM,
      styled,
      jsxRuntime,
      reactIs,
      propTypes,
      reactRouter,
    };

    const src = `${ASSETS_PREFIX}/${url}`;
    const script = document.createElement('script');
    script.src = src;
    script.onload = () => {
      if (!window.Cloud?.Cloud) {
        reject(
          new FetchError(
            `Failed to initialize Cloud: global export not found after loading ${src}`
          )
        );
        return;
      }
      resolve({ default: window.Cloud.Cloud });
    };
    script.onerror = () => {
      // script tag errors give us a bare DOM Event with no status or message —
      // the browser intentionally withholds that info. Use the URL so at least
      // the developer knows which asset failed; the Network tab has the rest.
      reject(new FetchError(`Failed to load Cloud assets: ${src}`));
    };

    document.body.appendChild(script);
  });
}
