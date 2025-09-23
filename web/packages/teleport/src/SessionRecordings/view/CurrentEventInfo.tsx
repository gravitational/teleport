import {
  useCallback,
  useImperativeHandle,
  useMemo,
  useRef,
  useState,
  type ReactNode,
  type Ref,
} from 'react';
import styled from 'styled-components';

import { ButtonPrimary } from 'design/Button';
import { FastForward } from 'design/Icon';

import {
  SessionRecordingEventType,
  type SessionRecordingEvent,
} from 'teleport/services/recordings';
import { formatSessionRecordingDuration } from 'teleport/SessionRecordings/list/RecordingItem';

export interface CurrentEventInfoHandle {
  setTime: (time: number) => void;
}

interface CurrentEventInfoProps {
  events: SessionRecordingEvent[];
  onSeek: (time: number) => void;
  ref?: Ref<CurrentEventInfoHandle>;
}

const EventsList = styled.div`
  display: flex;
  flex-direction: column;
  gap: ${props => props.theme.space[2]}px;
  position: absolute;
  top: ${props => props.theme.space[4]}px;
  right: ${props => props.theme.space[4]}px;
  z-index: 2;
`;

export function CurrentEventInfo({
  events,
  onSeek,
  ref,
}: CurrentEventInfoProps) {
  const [currentEvents, setCurrentEvents] = useState<SessionRecordingEvent[]>(
    []
  );
  const currentEventsRef = useRef<SessionRecordingEvent[]>([]);

  useImperativeHandle(ref, () => ({
    setTime(time: number) {
      const eventsInTimePeriod = events.filter(
        e => e.startTime <= time && e.endTime >= time
      );

      const hasChanged =
        eventsInTimePeriod.length !== currentEventsRef.current.length ||
        eventsInTimePeriod.some(
          (event, index) => event !== currentEventsRef.current[index]
        );

      if (hasChanged) {
        currentEventsRef.current = eventsInTimePeriod;
        setCurrentEvents(eventsInTimePeriod);
      }
    },
  }));

  const handleSkipToEnd = useCallback(
    (time: number) => {
      onSeek(time + 1);
    },
    [onSeek]
  );

  const items = useMemo(() => {
    if (currentEvents.length === 0) {
      return null;
    }

    const items: ReactNode[] = [];

    for (const [index, event] of currentEvents.entries()) {
      if (event.type !== SessionRecordingEventType.Inactivity) {
        continue;
      }

      items.push(
        <ButtonPrimary
          key={`event-${index}-${event.type}`}
          onClick={() => {
            handleSkipToEnd(event.endTime);
          }}
          px={2}
        >
          Skip {formatSessionRecordingDuration(event.endTime - event.startTime)}{' '}
          of inactivity
          <FastForward size="small" ml={2} />
        </ButtonPrimary>
      );
    }

    if (items.length === 0) {
      return null;
    }

    return items;
  }, [currentEvents, handleSkipToEnd]);

  if (currentEvents.length === 0) {
    return null;
  }

  return <EventsList>{items}</EventsList>;
}
