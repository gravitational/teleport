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

import React from 'react'
import { storiesOf } from '@storybook/react'
import { withInfo } from '@storybook/addon-info'
import Button, { ButtonPrimary, ButtonSecondary, ButtonWarning } from './Button';
import ButtonOutlinedDefault, * as ButtonOutlined from './../ButtonOutlined';
import ButtonIcon from './../ButtonIcon';
import ButtonLink from './../ButtonLink';
import { AddUsers, Trash, Ellipsis } from './../Icon';
import styled from 'styled-components';

storiesOf('Design/Button', module)
  .addDecorator(withInfo)
  .add('kinds', () => (
    <StyledBox>
      <ButtonPrimary>Primary</ButtonPrimary>
      <ButtonSecondary>Secondary</ButtonSecondary>
      <ButtonWarning >Warning</ButtonWarning>
      <ButtonLink href="" >Link Button</ButtonLink>
      <ButtonIcon> <AddUsers/></ButtonIcon>
      <ButtonIcon> <Ellipsis /></ButtonIcon>
      <ButtonIcon> <Trash /></ButtonIcon>
      <ButtonIcon> <Trash /></ButtonIcon>
      <ButtonOutlinedDefault>Default</ButtonOutlinedDefault>
      <ButtonOutlined.OutlinedPrimary>Primary</ButtonOutlined.OutlinedPrimary>
    </StyledBox>
  ))
  .add('sizes', () => (
    <div>
      <StyledBox>
        <Button size="large" mr={3}>Large</Button>
        <Button size="medium" mr={3}>Medium</Button>
        <Button size="small" mr={3}>Small</Button>
      </StyledBox>
      <StyledBox>
        <ButtonIcon size={2}> <AddUsers/></ButtonIcon>
        <ButtonIcon size={1}> <Ellipsis /></ButtonIcon>
        <ButtonIcon size={0}> <Trash /></ButtonIcon>
      </StyledBox>
  </div>
  ))
  .add('block', () => <Button block>Block Button</Button>)
  .add('states', () => (
    <StyledBox>
      <Button mr={3} disabled>Disabled</Button>
      <Button autoFocus> Focused</Button>
    </StyledBox>
  ));

const StyledBox = styled.div`
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  > * {
    margin-left: 20px;
    margin-bottom: 20px;
  }
`