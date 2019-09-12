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
import CardEmpty from 'gravity/components/CardEmpty';
import { Text } from 'design';
import * as Icons from 'design/Icon';
import ResourceCard from './../../components/ResourceCard';

export default function ConfigMapList({items, namespace, onEdit}){
  if(items.length === 0){
    return (
      <CardEmpty title="No Config Maps Found">
        <Text>
          There are no config maps for the "<Text as="span" bold>{namespace}</Text>" namespace
        </Text>
      </CardEmpty>
    )
  }

  return items.map(item => {
    const { name, created } = item;
    return (
      <ResourceCard
        buttonTitle="Edit config map"
        Icon={Icons.FileCode}
        key={name}
        name={name}
        created={created}
        onClick={() => onEdit(name)}
      />
    )
  })
}