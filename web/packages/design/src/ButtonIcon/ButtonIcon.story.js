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
import ButtonIcon from './ButtonIcon';
import { AddUsers, Trash, Ellipsis } from './../Icon';
import Flex from './../Flex';

export default {
  title: 'Design/Button',
};

export const Icons = () => (
  <>
    <Flex mb={3}>
      <ButtonIcon size={2}>
        <AddUsers />
      </ButtonIcon>
      <ButtonIcon size={2}>
        <Ellipsis />
      </ButtonIcon>
      <ButtonIcon size={2}>
        <Trash />
      </ButtonIcon>
    </Flex>
    <Flex mb={4}>
      <ButtonIcon size={1}>
        <AddUsers />
      </ButtonIcon>
      <ButtonIcon size={1}>
        <Ellipsis />
      </ButtonIcon>
      <ButtonIcon size={1}>
        <Trash />
      </ButtonIcon>
    </Flex>
    <Flex>
      <ButtonIcon size={0}>
        <AddUsers />
      </ButtonIcon>
      <ButtonIcon size={0}>
        <Ellipsis />
      </ButtonIcon>
      <ButtonIcon size={0}>
        <Trash />
      </ButtonIcon>
    </Flex>
  </>
);
