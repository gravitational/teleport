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
import styled from 'styled-components';

import { EditIcon } from '../../../Icons/EditIcon';
import { Type } from '../../../services/messages';

interface ActionProps {
  type: Type;
  value: string | string[];
}

const Container = styled.div`
  background: rgba(0, 0, 0, 0.2);
  border-radius: 10px;
  padding: 15px 20px;
  position: relative;
  width: 100%;
  box-sizing: border-box;
`;

const Title = styled.div`
  font-size: 15px;
  margin-bottom: 10px;
`;

const Items = styled.div`
  display: flex;
`;

const Item = styled.div`
  padding: 10px 15px;
  background: rgba(255, 255, 255, 0.1);
  border-radius: 5px;
  margin-right: 10px;
  font-size: 16px;
  font-family: SFMono-Regular, Consolas, Liberation Mono, Menlo, Courier,
    monospace;
`;

const Buttons = styled.div`
  position: absolute;
  right: 20px;
  top: 20px;
  opacity: 0.6;
`;

export function Action(props: ActionProps) {
  const items = [];

  if (Array.isArray(props.value)) {
    for (const [index, value] of props.value.entries()) {
      items.push(<Item key={index}>{value}</Item>);
    }
  } else {
    items.push(<Item key={0}>{props.value}</Item>);
  }

  return (
    <Container>
      <Title>{getTextForType(props.type)}</Title>

      <Buttons>
        <EditIcon size={18} />
      </Buttons>

      <Items>{items}</Items>
    </Container>
  );
}

function getTextForType(type: Type) {
  switch (type) {
    case Type.Connect:
      return 'Connect to';
    case Type.Exec:
      return 'Execute';
    case Type.Message:
      return '';
  }
}
