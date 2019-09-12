/*
Copyright 2019 Gravitational, Inc.

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

import { style, px, compose } from 'styled-system';

export const borderTopLeftRadius = style({
  prop: 'borderTopLeftRadius',
  key: 'radii',
  transformValue: px,
})

export const borderTopRightRadius = style({
  prop: 'borderTopRightRadius',
  key: 'radii',
  transformValue: px,
})

export const borderRadiusBottomRight = style({
  prop: 'borderBottomRightRadius',
  key: 'radii',
  transformValue: px,
})

export const borderBottomLeftRadius = style({
  prop: 'borderBottomLeftRadius',
  key: 'radii',
  transformValue: px,
})

export const borderRadius= style({
  prop: 'borderRadius',
  key: 'radii',
  transformValue: px,
})

const combined = compose(
  borderRadius,
  borderTopLeftRadius,
  borderTopRightRadius,
  borderRadiusBottomRight,
  borderBottomLeftRadius,
)

export default combined;