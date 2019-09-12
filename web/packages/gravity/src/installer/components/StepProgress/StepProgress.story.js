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
import { StepProgress } from './StepProgress';

storiesOf('GravityInstaller', module)
  .add('StepProgress', () => {
    const props = {
      ...defaultProps,
    }
    return (
      <StepProgress height="300px" {...props}
        logProvider={ <MockLogProvider /> }
      />
    )}
  )
  .add('StepProgress-Completed', () => {
    const props = {
      ...defaultProps,
      progress: {
        ...defaultProps.progress,
        isCompleted: true
      }
    }

    return (
      <StepProgress height="300px" {...props}
        logProvider={ <MockLogProvider /> }
      />
    )}
  )
  .add('StepProgress-Failed', () => {
    const props = {
      ...defaultProps,
      progress: {
        ...defaultProps.progress,
        isError: true
      }
    }

    return (
      <StepProgress height="300px" {...props}
        logProvider={ <MockLogProvider /> }
      />
    )}
  );

const defaultProps = {
  onFetch: $.Deferred(),
  progress: {
    siteId: 'siteId',
    opId: 'siteId',
    step: 3,
    isError: '',
    isCompleted: '',
    crashReportUrl: ''
  },
}

const MockLogProvider = () => {
  return null;
}