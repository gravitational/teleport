/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { useCallback, useMemo, useReducer, useRef } from 'react';

import {
  FileTransferListeners,
  TransferredFile,
  TransferState,
} from './FileTransferStateless';

type FilesStoreState = {
  ids: string[];
  filesById: Record<string, TransferredFile>;
};

type FilesStoreActions =
  | {
      type: 'add';
      payload: Pick<TransferredFile, 'id' | 'name'>;
    }
  | {
      type: 'updateTransferState';
      payload: {
        id: string;
        transferState: TransferState;
      };
    };

const initialState: FilesStoreState = {
  ids: [],
  filesById: {},
};

function reducer(
  state: typeof initialState,
  action: FilesStoreActions
): typeof initialState {
  switch (action.type) {
    case 'add': {
      return {
        ids: [action.payload.id, ...state.ids],
        filesById: {
          ...state.filesById,
          [action.payload.id]: {
            ...action.payload,
            transferState: { type: 'processing', progress: 0 },
          },
        },
      };
    }
    case 'updateTransferState': {
      const getNextTransferState = (): TransferState => {
        if (action.payload.transferState.type === 'error') {
          const { transferState: currentTransferState } =
            state.filesById[action.payload.id];
          return {
            ...action.payload.transferState,
            progress:
              currentTransferState.type === 'processing'
                ? currentTransferState.progress
                : 0,
          };
        }
        return action.payload.transferState;
      };

      return {
        ...state,
        filesById: {
          ...state.filesById,
          [action.payload.id]: {
            ...state.filesById[action.payload.id],
            transferState: getNextTransferState(),
          },
        },
      };
    }
    default:
      throw new Error('Unhandled action', action);
  }
}

export const useFilesStore = () => {
  const [state, dispatch] = useReducer(reducer, initialState);
  const abortControllers = useRef(new Map<string, AbortController>());

  const updateTransferState = useCallback(
    (id: string, transferState: TransferState) => {
      dispatch({ type: 'updateTransferState', payload: { id, transferState } });
    },
    []
  );

  const start = useCallback(
    async (options: {
      name: string;
      runFileTransfer(
        abortController: AbortController
      ): Promise<FileTransferListeners>;
    }) => {
      const abortController = new AbortController();
      const fileTransfer = await options.runFileTransfer(abortController);

      if (!fileTransfer) {
        return;
      }

      const id = new Date().getTime() + options.name;

      dispatch({ type: 'add', payload: { id, name: options.name } });
      abortControllers.current.set(id, abortController);

      fileTransfer.onProgress(progress => {
        updateTransferState(id, {
          type: 'processing',
          progress,
        });
      });
      fileTransfer.onError(error => {
        updateTransferState(id, {
          type: 'error',
          progress: undefined,
          error,
        });
      });
      fileTransfer.onComplete(() => {
        updateTransferState(id, {
          type: 'completed',
        });
      });
    },
    [updateTransferState]
  );

  const cancel = useCallback((id: string) => {
    abortControllers.current?.get(id).abort();
  }, []);

  const files = useMemo(
    () => state.ids.map(id => state.filesById[id]),
    [state.ids, state.filesById]
  );

  const isAnyTransferInProgress = useCallback(
    () => files.some(file => file.transferState.type === 'processing'),
    [files]
  );

  return {
    files,
    start,
    cancel,
    isAnyTransferInProgress,
  };
};

export type FilesStore = ReturnType<typeof useFilesStore>;
