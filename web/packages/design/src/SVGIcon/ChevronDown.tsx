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

export function ChevronDownIcon({ size = 14, fill }: SVGIconProps) {
  return (
    <SVGIcon viewBox="0 0 14 9" size={size} fill={fill}>
      <path d="M13.8889 1.65386L13.2629 1.02792C13.1148 0.879771 12.8746 0.879771 12.7264 1.02792L7 6.7407L1.27359 1.02795C1.12545 0.879802 0.885235 0.879802 0.737056 1.02795L0.111112 1.65389C-0.0370353 1.80204 -0.0370353 2.04225 0.111112 2.19043L6.73175 8.81101C6.8799 8.95916 7.12011 8.95916 7.26829 8.81101L13.8889 2.1904C14.037 2.04222 14.037 1.80201 13.8889 1.65386Z" />
    </SVGIcon>
  );
}
