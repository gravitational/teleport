/*
Copyright 2023 Gravitational, Inc.

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

import { SVGIcon } from './SVGIcon';

import type { SVGIconProps } from './common';

export function DevicesIcon({ size = 13, fill }: SVGIconProps) {
  return (
    <SVGIcon viewBox="0 0 1024 1024" size={size} fill={fill}>
      <path d="M179.2 716.8h665.6c42.342 0 76.8-34.458 76.8-76.8v-409.6c0-42.342-34.458-76.8-76.8-76.8h-665.6c-42.342 0-76.8 34.458-76.8 76.8v409.6c0 42.342 34.458 76.8 76.8 76.8zM153.6 230.4c0-14.131 11.469-25.6 25.6-25.6h665.6c14.131 0 25.6 11.469 25.6 25.6v409.6c0 14.131-11.469 25.6-25.6 25.6h-665.6c-14.131 0-25.6-11.469-25.6-25.6v-409.6z" />
      <path d="M998.4 768h-972.8c-14.131 0-25.6 11.469-25.6 25.6v51.2c0 42.342 34.458 76.8 76.8 76.8h870.4c42.342 0 76.8-34.458 76.8-76.8v-51.2c0-14.131-11.469-25.6-25.6-25.6zM947.2 870.4h-870.4c-14.131 0-25.6-11.469-25.6-25.6v-25.6h921.6v25.6c0 14.131-11.469 25.6-25.6 25.6z" />
    </SVGIcon>
  );
}
