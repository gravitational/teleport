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

export function TerminalIcon({ size = 32, fill }: SVGIconProps) {
  return (
    <SVGIcon fill={fill} size={size} viewBox="0 0 32 32">
      <path d="M0 2v28h32V2H0zm30 26H2V4h28v24zM28 6H4v20h24V6zM14 16h-2v2h-2v2H8v-2h2v-2h2v-2h-2v-2H8v-2h2v2h2v2h2v2zm8 4h-6v-2h6v2z" />
    </SVGIcon>
  );
}
