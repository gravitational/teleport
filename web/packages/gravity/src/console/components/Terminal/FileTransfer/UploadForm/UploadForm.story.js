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
import { storiesOf } from '@storybook/react';
import UploadForm from './UploadForm';

const defaultProps = {
  onUpload: () => null,
};

storiesOf('GravityConsole/Terminal/FileTransfer/UploadForm', module)
  .add('UploadForm', () => {
    return <UploadForm {...defaultProps} />;
  })
  .add('With selected files', () => <WithFiles />);

class WithFiles extends React.Component {
  componentDidMount() {
    const blobs = [{ length: 4343 }, { length: 4343 }];

    this.ref.addFiles([], blobs);
  }

  render() {
    return <UploadForm ref={e => (this.ref = e)} {...defaultProps} />;
  }
}
