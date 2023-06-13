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

export function ExpandIcon({ size = 16, fill }: SVGIconProps) {
  return (
    <SVGIcon fill={fill} size={size} viewBox="0 0 16 16">
      <path d="m15 5-1.284-1.284-3.29 3.29-1.432-1.432 3.29-3.29L11 1a1 1 0 0 1 1-1h3a1 1 0 0 1 1 1v3a1 1 0 0 1-1 1zM5 15a1 1 0 0 1-1 1H1a1 1 0 0 1-1-1v-3a1 1 0 0 1 1-1l1.284 1.284 3.29-3.29 1.432 1.432-3.29 3.291L5 15z" />
    </SVGIcon>
  );
}
