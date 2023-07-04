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

export function AnsibleIcon({ size = 800, fill }: SVGIconProps) {
  return (
    <SVGIcon viewBox="0 0 24 24" size={size} fill={fill}>
      <path d="m10.617 11.473 4.686 3.695-3.102-7.662zM12 0C5.371 0 0 5.371 0 12s5.371 12 12 12 12-5.371 12-12S18.629 0 12 0zm5.797 17.305a.851.851 0 0 1-.875.83c-.236 0-.416-.09-.664-.293l-6.19-5-2.079 5.203H6.191L11.438 5.44a.79.79 0 0 1 .764-.506.756.756 0 0 1 .742.506l4.774 11.494c.045.111.08.234.08.348l-.001.023z"/>
    </SVGIcon>
  );
}
