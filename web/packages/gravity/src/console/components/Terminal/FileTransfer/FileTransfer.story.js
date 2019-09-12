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
import FileTransfer from './FileTransfer';
import { FileTransferStore, File } from 'gravity/console/flux/scp/store';
import { Downloader } from 'gravity/console/services/fileTransfer';

const defaultProps = {
  onTransfer: () => null,
  onClose: () => null,
  onTransferRemove: () => null,
  onTransferUpdate: () => null,
  onTransferStart: () => null,
  store: new FileTransferStore({
    isOpen: true,
    login: 'root',
    serverId: '1d8d5c80-d74d-43bc-97e4-34da0554ff57',
    siteId: 'one',
  }),
};

const defaultFile = {
  location: '~test',
  id: '1547581437406~/test',
  url: '/v1/webapi/sites/one/nodes/',
  name:
    '~/test~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf~/mamaffsdfsdfdssdf',
  blob: [],
};

storiesOf('GravityConsole/Terminal/FileTransfer', module)
  .add('with download error', () => {
    const props = makeProps({
      isUpload: false,
      isFailed: true,
      error: 'stat /root/test: no such file or directory',
    });

    return (
      <MockDownloader>
        <FileTransfer {...props} />
      </MockDownloader>
    );
  })
  .add('with download progress', () => {
    const props = makeProps({
      isUpload: false,
      isProcessing: true,
    });

    return (
      <MockDownloader>
        <FileTransfer {...props} />
      </MockDownloader>
    );
  })
  .add('with download completed', () => {
    const props = makeProps({
      isUpload: false,
      isCompleted: true,
    });

    return (
      <MockDownloader>
        <FileTransfer {...props} />
      </MockDownloader>
    );
  })
  .add('with upload', () => {
    const props = {
      ...defaultProps,
      store: new FileTransferStore({ isOpen: true, isUpload: true }),
    };

    return (
      <MockDownloader>
        <FileTransfer {...props} />
      </MockDownloader>
    );
  });

const makeProps = json => {
  const file = new File({
    ...defaultFile,
    ...json,
  });

  const props = {
    ...defaultProps,
  };

  props.store = props.store.update('files', files => files.set(file.id, file));

  return props;
};

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
