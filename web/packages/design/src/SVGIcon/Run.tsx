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

import type { SVGIconProps } from './common';

export function RunIcon({ size = 48, fill = 'white' }: SVGIconProps) {
  return (
    <svg
      data-testid="svg"
      xmlns="http://www.w3.org/2000/svg"
      width={size}
      height={size}
      fill={fill}
      viewBox="0 0 64 64"
    >
      <path d="M26.023,43.745C25.467,43.283,25,42.724,25,42V23c0-0.734,0.402-1.41,1.048-1.759s1.431-0.316,2.046,0.084l15,9.798	c0.574,0.375,0.916,1.018,0.906,1.703c-0.01,0.686-0.37,1.318-0.954,1.676l-15,9.202C27.726,43.901,26.58,44.207,26.023,43.745z M29,26.695v11.731l9.262-5.682L29,26.695z" />
      <path d="M32,54c-12.131,0-22-9.869-22-22s9.869-22,22-22s22,9.869,22,22S44.131,54,32,54z M32,14	c-9.925,0-18,8.075-18,18s8.075,18,18,18s18-8.075,18-18S41.925,14,32,14z" />
    </svg>
  );
}
