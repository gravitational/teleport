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

import React from 'react';

import TopNav from './TopNav';
import TopNavItem from './TopNavItem';

export default {
  title: 'Design/TopNav',
};

export const Sample = () => {
  return (
    <TopNav height="60px">
      <TopNavItem>MenuItem1</TopNavItem>
      <TopNavItem>MenuItem2</TopNavItem>
    </TopNav>
  );
};
