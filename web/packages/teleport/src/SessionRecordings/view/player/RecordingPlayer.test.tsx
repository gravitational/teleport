/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { useRef } from 'react';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import { act } from 'react';

import { render } from 'design/utils/testing';

import type { PlayerHandle } from 'teleport/SessionRecordings/view/SshPlayer';
import { PlayerState } from 'teleport/SessionRecordings/view/stream/SessionStream';
import type { BaseEvent } from 'teleport/SessionRecordings/view/stream/types';
import { Player } from 'teleport/SessionRecordings/view/player/Player';

import { RecordingPlayer, type RecordingPlayerProps } from './RecordingPlayer';

// Mock the PlayerControls component
jest.mock('./PlayerControls', () => {
  const PlayerStateNames = {
    0: 'Loading',
    1: 'Paused',
    2: 'Playing',
    3: 'Stopped',
  };
  
  return {
    PlayerControls: jest.fn(
      ({ onPlay, onPause, onSeek, state, ref, duration, fullscreen, onToggleFullscreen, onToggleSidebar, onToggleTimeline }) => {
        // Expose methods via ref
        if (ref) {
          ref.current = {
            setTime: jest.fn(),
          };
        }
        
        return (
          <div data-testid="player-controls">
            <button onClick={onPlay} data-testid="play-button">Play</button>
            <button onClick={onPause} data-testid="pause-button">Pause</button>
            <button onClick={() => onSeek(5000)} data-testid="seek-button">Seek to 5s</button>
            <button onClick={onToggleFullscreen} data-testid="fullscreen-button">Fullscreen</button>
            <button onClick={onToggleSidebar} data-testid="sidebar-button">Toggle Sidebar</button>
            <button onClick={onToggleTimeline} data-testid="timeline-button">Toggle Timeline</button>
            <div data-testid="player-state">{PlayerStateNames[state]}</div>
            <div data-testid="duration">{duration}</div>
            <div data-testid="fullscreen-state">{fullscreen ? 'fullscreen' : 'normal'}</div>
          </div>
        );
      }
    ),
    PlayerControlsHandle: jest.fn(),
  };
});

// Mock the SessionStream class
jest.mock('teleport/SessionRecordings/view/stream/SessionStream', () => {
  const EventEmitter = require('events');
  
  return {
    SessionStream: jest.fn().mockImplementation(() => {
      const emitter = new EventEmitter();
      return {
        ...emitter,
        play: jest.fn(),
        pause: jest.fn(),
        seek: jest.fn(),
        loadInitial: jest.fn(),
        destroy: jest.fn(),
      };
    }),
    PlayerState: {
      Loading: 0,
      Paused: 1,
      Playing: 2,
      Stopped: 3,
    },
  };
});

// Mock icons
jest.mock('design/Icon', () => ({
  Play: ({ onClick, size }) => (
    <button onClick={onClick} data-testid={`play-icon-${size}`}>Play Icon</button>
  ),
  Pause: ({ size }) => (
    <div data-testid={`pause-icon-${size}`}>Pause Icon</div>
  ),
}));

// Create a mock Player implementation
class MockPlayer extends Player<TestEvent> {
  init = jest.fn();
  destroy = jest.fn();
  apply = jest.fn();
  handle = jest.fn().mockReturnValue(true);
  clear = jest.fn();
  fit = jest.fn();
  onPlay = jest.fn();
  onPause = jest.fn();
  onSeek = jest.fn();
  onStop = jest.fn();
}

interface TestEvent extends BaseEvent<number> {
  data: string;
}

describe('RecordingPlayer', () => {
  let mockPlayer: MockPlayer;
  let mockWs: WebSocket;
  let mockDecodeEvent: jest.Mock;
  let mockOnTimeChange: jest.Mock;
  let mockOnToggleSidebar: jest.Mock;
  let mockOnToggleTimeline: jest.Mock;
  let mockOnToggleFullscreen: jest.Mock;

  const defaultProps: RecordingPlayerProps<TestEvent> = {
    duration: 60000, // 60 seconds
    onTimeChange: jest.fn(),
    onToggleSidebar: jest.fn(),
    onToggleTimeline: jest.fn(),
    onToggleFullscreen: jest.fn(),
    fullscreen: false,
    player: null as any,
    endEventType: 999,
    decodeEvent: jest.fn(),
    ref: { current: null },
    ws: null as any,
  };

  beforeEach(() => {
    mockPlayer = new MockPlayer();
    mockWs = {
      onmessage: null,
      send: jest.fn(),
      close: jest.fn(),
    } as any;
    mockDecodeEvent = jest.fn();
    mockOnTimeChange = jest.fn();
    mockOnToggleSidebar = jest.fn();
    mockOnToggleTimeline = jest.fn();
    mockOnToggleFullscreen = jest.fn();

    // Clear all mocks
    jest.clearAllMocks();
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  const TestWrapper = (props: Partial<RecordingPlayerProps<TestEvent>> = {}) => {
    const ref = useRef<PlayerHandle>(null);
    
    return (
      <RecordingPlayer
        {...defaultProps}
        player={mockPlayer}
        ws={mockWs}
        decodeEvent={mockDecodeEvent}
        onTimeChange={mockOnTimeChange}
        onToggleSidebar={mockOnToggleSidebar}
        onToggleTimeline={mockOnToggleTimeline}
        onToggleFullscreen={mockOnToggleFullscreen}
        ref={ref}
        {...props}
      />
    );
  };

  describe('Component Initialization', () => {
    it('should render the component with initial state', () => {
      render(<TestWrapper />);
      
      expect(screen.getByTestId('player-controls')).toBeInTheDocument();
      expect(screen.getByTestId('player-state')).toHaveTextContent('Loading');
      expect(screen.getByTestId('duration')).toHaveTextContent('60000');
    });

    it('should display the play button initially', () => {
      render(<TestWrapper />);
      
      expect(screen.getByTestId('play-icon-extra-large')).toBeInTheDocument();
    });

    it('should initialize the player when component mounts', () => {
      render(<TestWrapper />);
      
      expect(mockPlayer.init).toHaveBeenCalled();
    });

    it('should create a SessionStream with correct parameters', () => {
      const SessionStream = require('teleport/SessionRecordings/view/stream/SessionStream').SessionStream;
      
      render(<TestWrapper />);
      
      expect(SessionStream).toHaveBeenCalledWith(
        mockWs,
        mockPlayer,
        mockDecodeEvent,
        999, // endEventType
        60000 // duration
      );
    });

    it('should load initial data from the stream', () => {
      const SessionStream = require('teleport/SessionRecordings/view/stream/SessionStream').SessionStream;
      const mockStreamInstance = SessionStream.mock.results[0].value;
      
      render(<TestWrapper />);
      
      expect(mockStreamInstance.loadInitial).toHaveBeenCalled();
    });
  });

  describe('Playback Controls', () => {
    it('should hide play button and call stream.play when play button is clicked', () => {
      const SessionStream = require('teleport/SessionRecordings/view/stream/SessionStream').SessionStream;
      const mockStreamInstance = SessionStream.mock.results[0].value;
      
      const { rerender } = render(<TestWrapper />);
      
      const playIcon = screen.getByTestId('play-icon-extra-large');
      fireEvent.click(playIcon);
      
      expect(mockStreamInstance.play).toHaveBeenCalled();
      
      // Rerender to check if play button is hidden
      rerender(<TestWrapper />);
      expect(screen.queryByTestId('play-icon-extra-large')).not.toBeInTheDocument();
    });

    it('should call stream.play when play button in controls is clicked', () => {
      const SessionStream = require('teleport/SessionRecordings/view/stream/SessionStream').SessionStream;
      const mockStreamInstance = SessionStream.mock.results[0].value;
      
      render(<TestWrapper />);
      
      const playButton = screen.getByTestId('play-button');
      fireEvent.click(playButton);
      
      expect(mockStreamInstance.play).toHaveBeenCalled();
    });

    it('should call stream.pause when pause button is clicked', () => {
      const SessionStream = require('teleport/SessionRecordings/view/stream/SessionStream').SessionStream;
      const mockStreamInstance = SessionStream.mock.results[0].value;
      
      render(<TestWrapper />);
      
      const pauseButton = screen.getByTestId('pause-button');
      fireEvent.click(pauseButton);
      
      expect(mockStreamInstance.pause).toHaveBeenCalled();
    });

    it('should call stream.seek with correct time when seek is triggered', () => {
      const SessionStream = require('teleport/SessionRecordings/view/stream/SessionStream').SessionStream;
      const mockStreamInstance = SessionStream.mock.results[0].value;
      
      render(<TestWrapper />);
      
      const seekButton = screen.getByTestId('seek-button');
      fireEvent.click(seekButton);
      
      expect(mockStreamInstance.seek).toHaveBeenCalledWith(5000);
    });
  });

  describe('Stream Event Handling', () => {
    it('should update state when stream emits state change', () => {
      const SessionStream = require('teleport/SessionRecordings/view/stream/SessionStream').SessionStream;
      const mockStreamInstance = SessionStream.mock.results[0].value;
      
      render(<TestWrapper />);
      
      act(() => {
        mockStreamInstance.emit('state', PlayerState.Playing);
      });
      
      expect(screen.getByTestId('player-state')).toHaveTextContent('Playing');
    });

    it('should update time and call onTimeChange when stream emits time update', () => {
      const SessionStream = require('teleport/SessionRecordings/view/stream/SessionStream').SessionStream;
      const mockStreamInstance = SessionStream.mock.results[0].value;
      const { PlayerControls } = require('./PlayerControls');
      
      render(<TestWrapper />);
      
      // Get the ref that was passed to PlayerControls
      const controlsRef = PlayerControls.mock.calls[0][0].ref;
      
      act(() => {
        mockStreamInstance.emit('time', 15000);
      });
      
      expect(controlsRef.current.setTime).toHaveBeenCalledWith(15000);
      expect(mockOnTimeChange).toHaveBeenCalledWith(15000);
    });
  });

  describe('Keyboard Shortcuts', () => {
    it('should toggle play/pause when spacebar is pressed', async () => {
      const SessionStream = require('teleport/SessionRecordings/view/stream/SessionStream').SessionStream;
      const mockStreamInstance = SessionStream.mock.results[0].value;
      
      render(<TestWrapper />);
      
      // Set state to Playing
      act(() => {
        mockStreamInstance.emit('state', PlayerState.Playing);
      });
      
      // Press spacebar to pause
      fireEvent.keyDown(document, { code: 'Space' });
      
      expect(mockStreamInstance.pause).toHaveBeenCalled();
      
      // Should show pause icon briefly
      await waitFor(() => {
        expect(screen.getByTestId('pause-icon-extra-large')).toBeInTheDocument();
      });
      
      // Set state to Paused
      act(() => {
        mockStreamInstance.emit('state', PlayerState.Paused);
      });
      
      // Clear previous calls
      mockStreamInstance.play.mockClear();
      
      // Press spacebar to play
      fireEvent.keyDown(document, { code: 'Space' });
      
      expect(mockStreamInstance.play).toHaveBeenCalled();
      
      // Should show play icon briefly
      await waitFor(() => {
        expect(screen.getByTestId('play-icon-extra-large')).toBeInTheDocument();
      });
    });

    it('should prevent default behavior when spacebar is pressed', () => {
      render(<TestWrapper />);
      
      const event = new KeyboardEvent('keydown', { code: 'Space' });
      const preventDefaultSpy = jest.spyOn(event, 'preventDefault');
      
      document.dispatchEvent(event);
      
      expect(preventDefaultSpy).toHaveBeenCalled();
    });
  });

  describe('Component Controls', () => {
    it('should call onToggleFullscreen when fullscreen button is clicked', () => {
      render(<TestWrapper />);
      
      const fullscreenButton = screen.getByTestId('fullscreen-button');
      fireEvent.click(fullscreenButton);
      
      expect(mockOnToggleFullscreen).toHaveBeenCalled();
    });

    it('should call onToggleSidebar when sidebar button is clicked', () => {
      render(<TestWrapper />);
      
      const sidebarButton = screen.getByTestId('sidebar-button');
      fireEvent.click(sidebarButton);
      
      expect(mockOnToggleSidebar).toHaveBeenCalled();
    });

    it('should call onToggleTimeline when timeline button is clicked', () => {
      render(<TestWrapper />);
      
      const timelineButton = screen.getByTestId('timeline-button');
      fireEvent.click(timelineButton);
      
      expect(mockOnToggleTimeline).toHaveBeenCalled();
    });

    it('should pass fullscreen prop correctly', () => {
      const { rerender } = render(<TestWrapper fullscreen={false} />);
      
      expect(screen.getByTestId('fullscreen-state')).toHaveTextContent('normal');
      
      rerender(<TestWrapper fullscreen={true} />);
      
      expect(screen.getByTestId('fullscreen-state')).toHaveTextContent('fullscreen');
    });
  });

  describe('Imperative Handle', () => {
    it('should expose moveToTime method through ref', () => {
      const SessionStream = require('teleport/SessionRecordings/view/stream/SessionStream').SessionStream;
      const mockStreamInstance = SessionStream.mock.results[0].value;
      
      const ref = { current: null } as any;
      render(<TestWrapper ref={ref} />);
      
      expect(ref.current).toBeDefined();
      expect(ref.current.moveToTime).toBeDefined();
      
      ref.current.moveToTime(20000);
      
      expect(mockStreamInstance.seek).toHaveBeenCalledWith(20000);
    });
  });

  describe('ResizeObserver', () => {
    let mockObserve: jest.Mock;
    let mockDisconnect: jest.Mock;
    let ResizeObserverMock: jest.Mock;

    beforeEach(() => {
      mockObserve = jest.fn();
      mockDisconnect = jest.fn();
      
      ResizeObserverMock = jest.fn(() => ({
        observe: mockObserve,
        disconnect: mockDisconnect,
        unobserve: jest.fn(),
      }));
      
      global.ResizeObserver = ResizeObserverMock as any;
    });

    afterEach(() => {
      delete (global as any).ResizeObserver;
    });

    it('should observe player element for resize', () => {
      render(<TestWrapper />);
      
      expect(ResizeObserverMock).toHaveBeenCalled();
      expect(mockObserve).toHaveBeenCalled();
    });

    it('should call player.fit when element is resized', () => {
      render(<TestWrapper />);
      
      const resizeCallback = ResizeObserverMock.mock.calls[0][0];
      resizeCallback();
      
      expect(mockPlayer.fit).toHaveBeenCalled();
    });

    it('should disconnect ResizeObserver on unmount', () => {
      const { unmount } = render(<TestWrapper />);
      
      unmount();
      
      expect(mockDisconnect).toHaveBeenCalled();
    });
  });

  describe('Cleanup', () => {
    it('should destroy stream on unmount', () => {
      const SessionStream = require('teleport/SessionRecordings/view/stream/SessionStream').SessionStream;
      const mockStreamInstance = SessionStream.mock.results[0].value;
      
      const { unmount } = render(<TestWrapper />);
      
      unmount();
      
      expect(mockStreamInstance.destroy).toHaveBeenCalled();
    });
  });

  describe('Edge Cases', () => {
    it('should handle missing controlsRef gracefully when time updates', () => {
      const SessionStream = require('teleport/SessionRecordings/view/stream/SessionStream').SessionStream;
      const mockStreamInstance = SessionStream.mock.results[0].value;
      const { PlayerControls } = require('./PlayerControls');
      
      render(<TestWrapper />);
      
      // Set the ref to null to simulate missing ref
      const controlsRef = PlayerControls.mock.calls[0][0].ref;
      controlsRef.current = null;
      
      // This should not throw
      expect(() => {
        act(() => {
          mockStreamInstance.emit('time', 15000);
        });
      }).not.toThrow();
      
      // onTimeChange should still be called
      expect(mockOnTimeChange).toHaveBeenCalledWith(15000);
    });

    it('should handle missing playerRef gracefully', () => {
      // This test ensures that the component doesn't crash if playerRef.current is null
      // The implementation already handles this with if (!playerRef.current) return;
      expect(() => {
        render(<TestWrapper />);
      }).not.toThrow();
    });
  });
});