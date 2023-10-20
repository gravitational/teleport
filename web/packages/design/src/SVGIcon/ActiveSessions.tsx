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

export function ActiveSessionsIcon({ size = 24, fill }: SVGIconProps) {
  return (
    <SVGIcon viewBox="0 0 24 20" size={size} fill={fill}>
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M22.2 19.6001H1.8C0.80775 19.6001 0 18.7924 0 17.8001V2.20015C0 1.2079 0.80775 0.400146 1.8 0.400146H22.2C23.1922 0.400146 24 1.2079 24 2.20015V17.8001C24 18.7924 23.1922 19.6001 22.2 19.6001ZM1.8 1.60015C1.4685 1.60015 1.2 1.86865 1.2 2.20015V17.8001C1.2 18.1316 1.4685 18.4001 1.8 18.4001H22.2C22.5315 18.4001 22.8 18.1316 22.8 17.8001V2.20015C22.8 1.86865 22.5315 1.60015 22.2 1.60015H1.8ZM4.20022 10.0001C4.00597 10.0001 3.81622 9.90639 3.69997 9.73239C3.51622 9.45639 3.59047 9.08439 3.86647 8.90064L6.71797 7.00014L3.86647 5.09964C3.59047 4.91589 3.51622 4.54314 3.69997 4.26789C3.88372 3.99264 4.25647 3.91764 4.53172 4.10139L8.13172 6.50139C8.29822 6.61314 8.39947 6.79989 8.39947 7.00089C8.39947 7.20189 8.29897 7.38864 8.13172 7.50039L4.53172 9.90039C4.42972 9.96864 4.31347 10.0009 4.19947 10.0009L4.20022 10.0001ZM10.1999 9.99993H13.7999C14.1314 9.99993 14.3999 9.73143 14.3999 9.39993C14.3999 9.06843 14.1314 8.79993 13.7999 8.79993H10.1999C9.86835 8.79993 9.59985 9.06843 9.59985 9.39993C9.59985 9.73143 9.86835 9.99993 10.1999 9.99993Z"
      />
    </SVGIcon>
  );
}
