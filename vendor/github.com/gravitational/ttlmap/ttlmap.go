package ttlmap

import (
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/mailgun/minheap"
)

// Option is a functional option argument to TTLMap
type Option func(m *TTLMap) error

// Clock sets the time provider clock, handy for testing
func Clock(clock clockwork.Clock) Option {
	return func(m *TTLMap) error {
		m.Clock = clock
		return nil
	}
}

// Callback can be executed on the element once TTLMap
// performs some action on the dictionary
type Callback func(key string, el interface{})

// CallOnExpire will call this callback on expiration of elements
func CallOnExpire(cb Callback) Option {
	return func(m *TTLMap) error {
		m.onExpire = cb
		return nil
	}
}

// TTLMap is a map with expiring elements
type TTLMap struct {
	capacity    int
	elements    map[string]*mapElement
	expiryTimes *minheap.MinHeap
	Clock       clockwork.Clock
	// onExpire callback will be called when element is expired
	onExpire Callback
}

type mapElement struct {
	key    string
	value  interface{}
	heapEl *minheap.Element
}

// New returns new instance of TTLMap
func New(capacity int, opts ...Option) (*TTLMap, error) {
	if capacity <= 0 {
		return nil, trace.BadParameter("capacity should be > 0")
	}

	m := &TTLMap{
		capacity:    capacity,
		elements:    make(map[string]*mapElement),
		expiryTimes: minheap.NewMinHeap(),
	}

	for _, o := range opts {
		if err := o(m); err != nil {
			return nil, err
		}
	}

	if m.Clock == nil {
		m.Clock = clockwork.NewRealClock()
	}

	return m, nil
}

// Pop removes and returns the key and the value of the oldest element
// from the TTL Map if there are no elements in the map, returns empty
// string and false
func (m *TTLMap) Pop() (string, interface{}, bool) {
	if len(m.elements) == 0 {
		return "", nil, false
	}
	heapEl := m.expiryTimes.PeekEl()
	mapEl := heapEl.Value.(*mapElement)
	m.Remove(mapEl.key)
	return mapEl.key, mapEl.value, true
}

// Remove removes and returns element if it's found,
// returns element, true if it's found
// returns nil, false if element is not found
func (m *TTLMap) Remove(key string) (interface{}, bool) {
	mapEl, exists := m.elements[key]
	if !exists {
		return nil, false
	}
	delete(m.elements, mapEl.key)
	m.expiryTimes.RemoveEl(mapEl.heapEl)
	return mapEl.value, true
}

// Set sets the value of the element by key to value and sets time to expire
// to TTL. Resolution is 1 second, min value is one second
func (m *TTLMap) Set(key string, value interface{}, ttl time.Duration) error {
	expiryTime, err := m.toEpochSeconds(ttl)
	if err != nil {
		return err
	}

	mapEl, exists := m.elements[key]
	if !exists {
		if len(m.elements) >= m.capacity {
			m.freeSpace(1)
		}
		heapEl := &minheap.Element{
			Priority: expiryTime,
		}
		mapEl := &mapElement{
			key:    key,
			value:  value,
			heapEl: heapEl,
		}
		heapEl.Value = mapEl
		m.elements[key] = mapEl
		m.expiryTimes.PushEl(heapEl)
	} else {
		mapEl.value = value
		m.expiryTimes.UpdateEl(mapEl.heapEl, expiryTime)
	}
	return nil
}

func (m *TTLMap) toEpochSeconds(ttl time.Duration) (int, error) {
	if ttl < time.Second {
		return 0, trace.BadParameter("ttl should be >= time.Second, got %v", ttl)
	}
	return int(m.Clock.Now().UTC().Add(ttl).Unix()), nil
}

// Len returns amount of elements in the map
func (m *TTLMap) Len() int {
	return len(m.elements)
}

// Get returns element value, does not update TTL
func (m *TTLMap) Get(key string) (interface{}, bool) {
	mapEl, exists := m.elements[key]
	if !exists {
		return nil, false
	}
	if m.expireElement(mapEl) {
		return nil, false
	}
	return mapEl.value, true
}

// Increment increment element's value and updates it's TTL
func (m *TTLMap) Increment(key string, value int, ttl time.Duration) (int, error) {
	expiryTime, err := m.toEpochSeconds(ttl)
	if err != nil {
		return 0, err
	}

	mapEl, exists := m.elements[key]
	if !exists {
		m.Set(key, value, ttl)
		return value, nil
	}
	if m.expireElement(mapEl) {
		m.Set(key, value, ttl)
		return value, nil
	}
	currentValue, ok := mapEl.value.(int)
	if !ok {
		return 0, trace.BadParameter("expected existing value to be integer, got %T", mapEl.value)
	}
	currentValue += value
	mapEl.value = currentValue

	m.expiryTimes.UpdateEl(mapEl.heapEl, expiryTime)
	return currentValue, nil
}

// GetInt returns integer element value stored in map, does not update TTL
func (m *TTLMap) GetInt(key string) (int, bool, error) {
	valueI, exists := m.Get(key)
	if !exists {
		return 0, false, nil
	}
	value, ok := valueI.(int)
	if !ok {
		return 0, false, trace.BadParameter("expected existing value to be integer, got %T", valueI)
	}
	return value, true, nil
}

func (m *TTLMap) expireElement(mapEl *mapElement) bool {
	now := int(m.Clock.Now().UTC().Unix())
	if mapEl.heapEl.Priority > now {
		return false
	}

	if m.onExpire != nil {
		m.onExpire(mapEl.key, mapEl.value)
	}

	delete(m.elements, mapEl.key)
	m.expiryTimes.RemoveEl(mapEl.heapEl)
	return true
}

func (m *TTLMap) freeSpace(count int) {
	removed := m.RemoveExpired(count)
	if removed >= count {
		return
	}
	m.removeLastUsed(count - removed)
}

// RemoveExpired makes a pass through map and removes expired elements
func (m *TTLMap) RemoveExpired(iterations int) int {
	removed := 0
	now := int(m.Clock.Now().UTC().Unix())
	for i := 0; i < iterations; i++ {
		if len(m.elements) == 0 {
			break
		}
		heapEl := m.expiryTimes.PeekEl()
		if heapEl.Priority > now {
			break
		}
		m.expiryTimes.PopEl()
		mapEl := heapEl.Value.(*mapElement)
		if m.onExpire != nil {
			m.onExpire(mapEl.key, mapEl.value)
		}
		delete(m.elements, mapEl.key)
		removed++
	}
	return removed
}

func (m *TTLMap) removeLastUsed(iterations int) {
	for i := 0; i < iterations; i++ {
		if len(m.elements) == 0 {
			return
		}
		heapEl := m.expiryTimes.PopEl()
		mapEl := heapEl.Value.(*mapElement)
		if m.onExpire != nil {
			m.onExpire(mapEl.key, mapEl.value)
		}
		delete(m.elements, mapEl.key)
	}
}
