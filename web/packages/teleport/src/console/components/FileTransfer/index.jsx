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
import { withState } from 'shared/hooks';
import useScpContext, { useStoreFiles } from './useScpContext';
import FileTransfer from './FileTransfer';

export default withState(props => {
  const {
    clusterId,
    serverId,
    login,
    isDownloadOpen,
    isUploadOpen,
    onClose,
  } = props;

  const scpContext = useScpContext();

  // re-init the context each time when props change
  React.useMemo(() => {
    scpContext.init({
      clusterId,
      serverId,
      login,
    });
  }, [isDownloadOpen, isUploadOpen, clusterId, serverId, login]);

  // subscribe to store updates
  const storeFiles = useStoreFiles();

  function onRemove(id) {
    scpContext.removeFile(id);
  }

  function onUpdate(json) {
    scpContext.updateFile(json);
  }

  function onDownload(location) {
    scpContext.addDownload(location);
  }

  function onUpload(location, filename, blob) {
    scpContext.addUpload(location, filename, blob);
  }

  function onBeforeClose() {
    const isTransfering = scpContext.isTransfering();
    if (!isTransfering) {
      onClose();
    }

    if (
      isTransfering &&
      window.confirm('Are you sure you want to cancel file transfers?')
    ) {
      onClose();
    }
  }

  return {
    isDownloadOpen,
    isUploadOpen,
    onDownload,
    onUpload,
    onRemove,
    onUpdate,
    onClose: onBeforeClose,
    files: storeFiles.state.files,
  };
})(FileTransfer);
