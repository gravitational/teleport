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

import {
  act,
  fireEvent,
  render,
  screen,
  waitForElementToBeRemoved,
} from 'design/utils/testing';

import { createFileTransferEventsEmitter } from './createFileTransferEventsEmitter';
import { FileTransfer, TransferHandlers } from './FileTransfer';
import { FileTransferContextProvider } from './FileTransferContextProvider';
import { FileTransferDialogDirection } from './FileTransferStateless';

function FileTransferTestWrapper(props: {
  beforeClose?: () => boolean | Promise<boolean>;
  afterClose?: () => void;
  transferHandlers: TransferHandlers;
}) {
  return (
    <FileTransferContextProvider
      openedDialog={FileTransferDialogDirection.Download}
    >
      <FileTransfer {...props} />
    </FileTransferContextProvider>
  );
}

test('click opens correct dialog', () => {
  render(
    <FileTransferTestWrapper
      beforeClose={undefined}
      transferHandlers={undefined}
    />
  );
  expect(screen.getByText('Download Files')).toBeInTheDocument();
});

test('downloads component changes when file transfer callbacks are called', async () => {
  const fileTransferEvents = createFileTransferEventsEmitter();

  const handler: TransferHandlers = {
    getDownloader: async () => fileTransferEvents,
    getUploader: async () => undefined,
  };
  render(
    <FileTransferTestWrapper
      beforeClose={undefined}
      transferHandlers={handler}
    />
  );
  fireEvent.change(screen.getByLabelText('File Path'), {
    target: { value: '/Users/g/file.txt' },
  });
  fireEvent.click(screen.getByText('Download'));
  const listItem = await screen.findByRole('listitem');
  expect(listItem).toHaveTextContent('/Users/g/file.txt');

  act(() => fileTransferEvents.emitProgress(50));
  expect(listItem).toHaveTextContent('50%');

  act(() => fileTransferEvents.emitComplete());
  expect(listItem).toContainElement(screen.getByTitle('Transfer completed'));

  act(() => fileTransferEvents.emitError(new Error('Network error')));
  expect(listItem).toHaveTextContent('Network error');
});

test('onAbort is called when user cancels upload', async () => {
  let abortControllerMock: AbortController;

  const handler: TransferHandlers = {
    getDownloader: async (_, abortController) => {
      abortControllerMock = abortController;
      return createFileTransferEventsEmitter();
    },
    getUploader: async () => undefined,
  };
  render(
    <FileTransferTestWrapper
      beforeClose={undefined}
      transferHandlers={handler}
    />
  );
  fireEvent.change(screen.getByLabelText('File Path'), {
    target: { value: '/Users/g/file.txt' },
  });
  fireEvent.click(screen.getByText('Download'));
  fireEvent.click(await screen.findByTitle('Cancel'));
  expect(abortControllerMock.signal.aborted).toBeTruthy();
});

test('file is not added when transferHandler does not return anything', async () => {
  const handler: TransferHandlers = {
    getDownloader: async () => undefined,
    getUploader: async () => undefined,
  };
  const filePath = '/Users/g/file.txt';

  render(
    <FileTransferTestWrapper
      beforeClose={undefined}
      transferHandlers={handler}
    />
  );
  fireEvent.change(screen.getByLabelText('File Path'), {
    target: { value: filePath },
  });
  fireEvent.click(screen.getByText('Download'));
  expect(screen.queryByText('/Users/g/file.txt')).not.toBeInTheDocument();
});

describe('handleAfterClose', () => {
  const getSetup = async () => {
    const handleBeforeClose = jest.fn();
    const handleAfterClose = jest.fn();
    const handler: TransferHandlers = {
      getDownloader: async () => createFileTransferEventsEmitter(),
      getUploader: async () => undefined,
    };

    render(
      <FileTransferTestWrapper
        beforeClose={handleBeforeClose}
        afterClose={handleAfterClose}
        transferHandlers={handler}
      />
    );

    fireEvent.change(screen.getByLabelText('File Path'), {
      target: { value: '~/abc' },
    });

    fireEvent.click(screen.getByText('Download'));
    await screen.findByRole('listitem');

    return { handleBeforeClose, handleAfterClose };
  };

  test('is not called when closing the dialog has been aborted', async () => {
    const { handleBeforeClose, handleAfterClose } = await getSetup();
    handleBeforeClose.mockReturnValue(Promise.resolve(false));
    fireEvent.click(screen.getByTitle('Close'));
    expect(handleBeforeClose).toHaveBeenCalled();
    expect(handleAfterClose).not.toHaveBeenCalled();
  });

  test('is called when closing the dialog has been confirmed', async () => {
    const { handleBeforeClose, handleAfterClose } = await getSetup();
    handleBeforeClose.mockReturnValue(Promise.resolve(true));
    fireEvent.click(screen.getByTitle('Close'));
    expect(handleBeforeClose).toHaveBeenCalled();

    // wait for dialog to close
    await waitForElementToBeRemoved(() =>
      screen.queryByTestId('file-transfer-container')
    );
    expect(handleAfterClose).toHaveBeenCalled();
  });
});
