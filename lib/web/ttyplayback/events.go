/**
 * Copyright (C) 2024 Gravitational, Inc.
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

package ttyplayback

type Event struct {
	Type string `json:"type"`
	Data string `json:"data"`
	Time int64  `json:"time"`
}

type EventIterator interface {
	Next() (*Event, bool)
}

type SliceIterator struct {
	events []Event
	index  int
}

func (s *SliceIterator) Next() (*Event, bool) {
	if s.index >= len(s.events) {
		return nil, false
	}
	event := &s.events[s.index]
	s.index++
	return event, true
}

type Batch struct {
	iter         EventIterator
	prevTime     int64
	prevData     string
	prevType     string
	maxFrameTime float64
	isFirst      bool
}

func (b *Batch) Next() (*Event, bool) {
	if b.isFirst {
		event, ok := b.iter.Next()
		if !ok {
			return nil, false
		}
		b.prevTime = event.Time
		b.prevData = event.Data
		b.prevType = event.Type
		b.isFirst = false
	}

	for {
		event, ok := b.iter.Next()
		if !ok {
			if b.prevData != "" {
				prevTime := b.prevTime
				prevData := b.prevData
				prevType := b.prevType
				b.prevData = ""
				return &Event{Time: prevTime, Data: prevData, Type: prevType}, true
			}
			return nil, false
		}

		if float64(event.Time-b.prevTime) < b.maxFrameTime {
			b.prevData += event.Data
		} else {
			prevTime := b.prevTime
			prevData := b.prevData
			prevType := b.prevType
			b.prevTime = event.Time
			b.prevData = event.Data
			b.prevType = event.Type
			return &Event{Time: prevTime, Data: prevData, Type: prevType}, true
		}
	}
}

func BatchEvents(iter EventIterator, fpsCap uint8) EventIterator {
	return &Batch{
		iter:         iter,
		prevData:     "",
		prevType:     "",
		prevTime:     0.0,
		maxFrameTime: 1.0 / float64(fpsCap),
		isFirst:      true,
	}
}

type AccelerateIterator struct {
	iter  EventIterator
	speed float64
}

func (a *AccelerateIterator) Next() (*Event, bool) {
	event, ok := a.iter.Next()
	if !ok {
		return nil, false
	}
	return &Event{Time: event.Time, Data: event.Data, Type: event.Type}, true
}

func Accelerate(iter EventIterator, speed float64) EventIterator {
	return &AccelerateIterator{
		iter:  iter,
		speed: speed,
	}
}

type LimitIdleTimeIterator struct {
	iter     EventIterator
	limit    int64
	prevTime int64
	offset   int64
}

func (l *LimitIdleTimeIterator) Next() (*Event, bool) {
	event, ok := l.iter.Next()
	if !ok {
		return nil, false
	}

	delay := event.Time - l.prevTime
	excess := delay - l.limit

	if excess > 0 {
		l.offset += excess
	}

	l.prevTime = event.Time

	return &Event{Time: event.Time - l.offset, Data: event.Data, Type: event.Type}, true
}

func LimitIdleTime(iter EventIterator, limit int64) EventIterator {
	return &LimitIdleTimeIterator{
		iter:     iter,
		limit:    limit,
		prevTime: 0.0,
		offset:   0.0,
	}
}

func CollectAll(iter EventIterator) []Event {
	var events []Event
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		events = append(events, *event)
	}
	return events
}

func NewSliceIterator(events []Event) EventIterator {
	return &SliceIterator{events: events, index: 0}
}

type PrintEventsIterator struct {
	iter EventIterator
}

func (p *PrintEventsIterator) Next() (*Event, bool) {
	event, ok := p.iter.Next()
	if !ok {
		return nil, false
	}

	if event.Type == "print" {
		return event, true
	}

	// Skip non-print events
	return p.Next()
}

func OnlyPrintEvents(iter EventIterator) EventIterator {
	return &PrintEventsIterator{iter: iter}
}
