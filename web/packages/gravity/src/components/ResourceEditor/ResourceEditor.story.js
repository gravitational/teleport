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
import $ from 'jQuery';
import { storiesOf } from '@storybook/react'
import ResourceEditor from './ResourceEditor';

storiesOf('Gravity/ResourceEditor', module)
  .add('New', () => (
    <ResourceEditor
      resource={{
        isNew: true,
        name: 'sample',
        content: 'fdfd'
      }}
      title="Creating a new resource"
      name="sample"
      onSave={() => {
        return $.Deferred() }
      }
      open={true} />
  ))
  .add('Edit', () => (
    <ResourceEditor
      resource={{
        isNew: false,
        name: 'sample',
        content: 'fdfd'
      }}
      title="Modifying the resource"
      name="sample"
      onSave={() => $.Deferred() }
      open={true} />
  ))
  .add('Directions', () => (
    <ResourceEditor
      resource={{
        isNew: false,
        name: 'sample',
        content: 'fdfd'
      }}
      docsURL="localhost"
      directions={ <div>fsdfdfs</div>}
      title="Modifying the resource"
      name="sample"
      onSave={() => $.Deferred() }
      open={true} />
  ));