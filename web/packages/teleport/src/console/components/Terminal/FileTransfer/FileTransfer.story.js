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
import { Downloader } from 'teleport/console/services/fileTransfer';
import FileTransfer from './FileTransfer';
import { StoreScp } from '../../../stores';

function createProps() {
  return {
    onTransfer: () => null,
    onClose: () => null,
    onTransferRemove: () => null,
    onTransferUpdate: () => null,
    onTransferStart: () => null,
    store: new StoreScp({
      isOpen: true,
      clusterId: '3',
      serverId: '255',
      login: 'root',
    }),
  };
}

const defaultFile = {
  location: '~test',
  id: '1547581437406~/test',
  url: '/v1/webapi/sites/one/nodes/',
  name:
    '~/test~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf',
  blob: [],
};

storiesOf('TeleportConsole/Terminal/FileTransfer', module)
  .add('with download error', () => {
    const props = createProps();
    const file = {
      ...defaultFile,
      isFailed: true,
      error: 'stat /root/test: no such file or directory',
    };

    props.store.setState({
      isOpen: true,
      isUpload: false,
      files: [file],
    });

    return (
      <MockDownloader>
        <FileTransfer {...props} />
      </MockDownloader>
    );
  })
  .add('with download progress', () => {
    const props = createProps();
    const file = {
      ...defaultFile,
      isProcessing: true,
    };

    props.store.setState({
      isOpen: true,
      isUpload: false,
      files: [file],
    });

    return (
      <MockDownloader>
        <FileTransfer {...props} />
      </MockDownloader>
    );
  })
  .add('with download completed', () => {
    const props = createProps();
    const file = {
      ...defaultFile,
      isCompleted: true,
    };

    props.store.setState({
      isOpen: true,
      isUpload: false,
      files: [file],
    });

    return (
      <MockDownloader>
        <FileTransfer {...props} />
      </MockDownloader>
    );
  })
  .add('with upload', () => {
    const props = createProps();
    props.store = new StoreScp({ isOpen: true, isUpload: true });
    return (
      <MockDownloader>
        <FileTransfer {...props} />
      </MockDownloader>
    );
  });

class MockDownloader extends React.Component {
  constructor(props) {
    super(props);
    this.original = Downloader.prototype.do;
    Downloader.prototype.do = () => null;
  }

  componentWillUnmount() {
    Downloader.prototype.do = this.original;
  }

  render() {
    return this.props.children;
  }
}
