import { ButtonState, TdpClient } from 'teleport/lib/tdp';

import { Withholder } from './Withholder';

// Mock the TdpClient class
jest.mock('teleport/lib/tdp', () => {
  const originalModule = jest.requireActual('teleport/lib/tdp'); // Get the actual module
  return {
    ...originalModule,
    TdpClient: jest.fn().mockImplementation(() => {
      return {
        connect: jest.fn().mockResolvedValue(undefined),
      };
    }),
  };
});

describe('withholder', () => {
  jest.useFakeTimers();
  let withholder: Withholder;
  let mockHandleKeyboardEvent: jest.Mock;

  beforeEach(() => {
    withholder = new Withholder();
    mockHandleKeyboardEvent = jest.fn();
  });

  afterEach(() => {
    withholder.cancel();
    mockHandleKeyboardEvent.mockClear();
    jest.clearAllTimers();
  });

  it('handles non-withheld keys immediately', () => {
    const params = {
      e: { key: 'Enter' } as KeyboardEvent as KeyboardEvent,
      state: ButtonState.DOWN,
      cli: new TdpClient('wss://socketAddr.gov'),
    };
    withholder.handleKeyboardEvent(params, mockHandleKeyboardEvent);
    expect(mockHandleKeyboardEvent).toHaveBeenCalledWith(params);
  });

  it('flushes withheld keys upon non-withheld key press', () => {
    const metaDown = {
      e: { key: 'Meta' } as KeyboardEvent,
      state: ButtonState.DOWN,
      cli: new TdpClient('wss://socketAddr.gov'),
    };

    const metaUp = {
      e: { key: 'Meta' } as KeyboardEvent,
      state: ButtonState.UP,
      cli: new TdpClient('wss://socketAddr.gov'),
    };

    const enterDown = {
      e: { key: 'Enter' } as KeyboardEvent as KeyboardEvent,
      state: ButtonState.DOWN,
      cli: new TdpClient('wss://socketAddr.gov'),
    };

    withholder.handleKeyboardEvent(metaDown, mockHandleKeyboardEvent);
    withholder.handleKeyboardEvent(metaUp, mockHandleKeyboardEvent);

    expect(mockHandleKeyboardEvent).not.toHaveBeenCalled();

    withholder.handleKeyboardEvent(enterDown, mockHandleKeyboardEvent);

    expect(mockHandleKeyboardEvent).toHaveBeenCalledTimes(3);
    expect(mockHandleKeyboardEvent).toHaveBeenNthCalledWith(1, metaDown);
    expect(mockHandleKeyboardEvent).toHaveBeenNthCalledWith(2, metaUp);
    expect(mockHandleKeyboardEvent).toHaveBeenNthCalledWith(3, enterDown);
  });

  it('withholds and then handles Meta and Alt keys UP on a timer', () => {
    const metaParams = {
      e: { key: 'Meta' } as KeyboardEvent,
      state: ButtonState.UP,
      cli: new TdpClient('wss://socketAddr.gov'),
    };
    const altParams = {
      e: { key: 'Alt' } as KeyboardEvent,
      state: ButtonState.UP,
      cli: new TdpClient('wss://socketAddr.gov'),
    };

    withholder.handleKeyboardEvent(metaParams, mockHandleKeyboardEvent);
    withholder.handleKeyboardEvent(altParams, mockHandleKeyboardEvent);

    expect(mockHandleKeyboardEvent).not.toHaveBeenCalled();

    jest.advanceTimersByTime(10);

    expect(mockHandleKeyboardEvent).toHaveBeenCalledTimes(2);
    expect(mockHandleKeyboardEvent).toHaveBeenNthCalledWith(1, metaParams);
    expect(mockHandleKeyboardEvent).toHaveBeenNthCalledWith(2, altParams);
  });

  it('cancels withheld keys correctly', () => {
    const metaParams = {
      e: { key: 'Meta' } as KeyboardEvent,
      state: ButtonState.UP,
      cli: new TdpClient('wss://socketAddr.gov'),
    };
    withholder.handleKeyboardEvent(metaParams, mockHandleKeyboardEvent);

    withholder.cancel();

    jest.advanceTimersByTime(10);

    expect(mockHandleKeyboardEvent).not.toHaveBeenCalled();
    expect((withholder as any).withheldKeys).toHaveLength(0);
  });
});
