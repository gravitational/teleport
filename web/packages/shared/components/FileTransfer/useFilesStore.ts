/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
