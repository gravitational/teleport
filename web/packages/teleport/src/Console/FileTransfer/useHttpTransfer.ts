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
import useScpContext from './scpContextProvider';
import { FileState } from './types';

export default function useHttpTransfer({ blob, url, isUpload }) {
  const { createDownloader, createUploader } = useScpContext();
  const [http] = React.useState(() => {
    return isUpload ? createUploader() : createDownloader();
  });

  const [status, setStatus] = React.useState({
    response: null,
    progress: 0,
    state: 'processing' as FileState,
    error: '',
  });

  React.useEffect(() => {
    function handleProgress(progress: number) {
      setStatus({
        ...status,
        progress,
      });
    }

    function handleCompleted(response) {
      setStatus({
        ...status,
        response,
        state: 'completed',
      });
    }

    function handleFailed(err) {
      setStatus({
        ...status,
        error: err.message,
        state: 'error',
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
