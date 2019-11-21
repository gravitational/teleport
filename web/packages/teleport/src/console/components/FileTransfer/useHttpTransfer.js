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
import useScpContext, { FileStateEnum } from './useScpContext';

export default function useHttpTransfer({ blob, url, isUpload }) {
  const { createDownloader, createUploader } = useScpContext();
  const [http] = React.useState(() => {
    return isUpload ? createUploader() : createDownloader();
  });

  const [status, setStatus] = React.useState({
    response: null,
    progress: '0',
    state: FileStateEnum.PROCESSING,
    error: '',
  });

  React.useEffect(() => {
    function handleProgress(progress) {
      setStatus({
        ...status,
        progress,
      });
    }

    function handleCompleted(response) {
      setStatus({
        ...status,
        response,
        state: FileStateEnum.COMPLETED,
      });
    }

    function handleFailed(err) {
      setStatus({
        ...status,
        error: err.message,
        state: FileStateEnum.ERROR,
      });
    }

    http.onProgress(handleProgress);
    http.onCompleted(handleCompleted);
    http.onError(handleFailed);
    http.do(url, blob);

    function cleanup() {
      http.removeAllListeners();
      http.abort();
    }

    return cleanup;
  }, []);

  return status;
}

export function isProcessing(value) {
  return value === FileStateEnum.PROCESSING;
}

export function isError(value) {
  return value === FileStateEnum.ERROR;
}

export function isCompleted(value) {
  return value === FileStateEnum.COMPLETED;
}
