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

export function ChevronRightIcon({ size = 14, fill }: SVGIconProps) {
  return (
    <SVGIcon viewBox="0 0 9 14" size={size} fill={fill}>
      <path d="M1.65386 0.111133L1.02792 0.737077C0.879771 0.885224 0.879771 1.12543 1.02792 1.27361L6.7407 7L1.02795 12.7264C0.8798 12.8746 0.8798 13.1148 1.02795 13.2629L1.65389 13.8889C1.80204 14.037 2.04225 14.037 2.19043 13.8889L8.81101 7.26825C8.95915 7.1201 8.95915 6.87989 8.81101 6.73171L2.1904 0.111133C2.04222 -0.0370463 1.80201 -0.0370464 1.65386 0.111133Z" />
    </SVGIcon>
  );
}
