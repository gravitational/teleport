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

export default function useFileTransferDialogs() {
  const [isUploadOpen, setUploadOpen] = React.useState(false);
  const [isDownloadOpen, setDownloadOpen] = React.useState(false);

  function openDownload() {
    setDownloadOpen(true);
  }

  function openUpload() {
    setUploadOpen(true);
  }

  function close() {
    setUploadOpen(false);
    setDownloadOpen(false);
  }

  return {
    isUploadOpen,
    isDownloadOpen,
    close,
    openDownload,
    openUpload,
  };
}
