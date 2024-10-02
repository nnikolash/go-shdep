package updtree

import "fmt"

// NOTE: Not thread safe

type AccumulatedEvent[Event any] struct {
	Event     *Event
	readTimes int
}

func NewEventsPullStorage[Event any]() *EventsPullStorage[Event] {
	return &EventsPullStorage[Event]{}
}

type EventsPullStorage[Event any] struct {
	eventsPushed int
	events       []AccumulatedEvent[Event]
	pullersCount int
}

func (a *EventsPullStorage[Event]) NewPuller() *EventPuller[Event] {
	a.pullersCount++
	return &EventPuller[Event]{acc: a}
}

func (a *EventsPullStorage[Event]) Publish(evt Event) {
	if a.pullersCount == 0 {
		return
	}

	a.eventsPushed++
	a.events = append(a.events, AccumulatedEvent[Event]{
		Event:     &evt,
		readTimes: 0,
	})
}

func (a *EventsPullStorage[Event]) getEvents(from int) []AccumulatedEvent[Event] {
	countToPull := a.eventsPushed - from
	if countToPull == 0 {
		return nil
	}

	firstEvtIdx := len(a.events) - countToPull
	if firstEvtIdx < 0 {
		panic(fmt.Errorf("invalid event index: %v: current len = %v, eventsPushed = %v, from = %v",
			firstEvtIdx, len(a.events), a.eventsPushed, from))
	}

	pulledEvents := a.events[firstEvtIdx:len(a.events):len(a.events)]
	for i := range pulledEvents {
		pulledEvents[i].readTimes++
	}

	countToErase := 0
	for i := 0; i < len(a.events); i++ {
		if a.events[i].readTimes != a.pullersCount {
			break
		}
		countToErase++
	}

	if countToErase > 0 {
		a.events = a.events[countToErase:]
	}

	return pulledEvents
}

func (a *EventsPullStorage[Event]) Len() int {
	return a.BufferSize()
}

func (a *EventsPullStorage[Event]) BufferSize() int {
	return len(a.events)
}

func (a *EventsPullStorage[Event]) EventsPushed() int {
	return a.eventsPushed
}

type EventPuller[Event any] struct {
	acc    *EventsPullStorage[Event]
	cursor int
}

func (p *EventPuller[Event]) Pull() []AccumulatedEvent[Event] {
	events := p.acc.getEvents(p.cursor)
	if len(events) == 0 {
		return nil
	}

	p.cursor += len(events)

	return events
}
