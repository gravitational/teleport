package goskipiter

import "github.com/ryszard/goskiplist/skiplist"

// Iterator wraps goskiplist's iterator, which is a bit janky; seeking doesn't
// play nice with the iteration idiom. If you seek, then iterate using the
// examples provided in the godoc, your iteration will always skip the first
// result. It would be less error prone and astonishing if Seek meant that the
// next call to Next() would give you what you expect.
type Iterator struct {
	inner     skiplist.Iterator
	didSeek   bool
	seekWasOK bool
}

func New(inner skiplist.Iterator) *Iterator {
	return &Iterator{inner: inner}
}

// Next returns true if the iterator contains subsequent elements
// and advances its state to the next element if that is possible.
func (iter *Iterator) Next() (ok bool) {
	if iter.didSeek {
		iter.didSeek = false
		return iter.seekWasOK
	} else {
		return iter.inner.Next()
	}
}

// Previous returns true if the iterator contains previous elements
// and rewinds its state to the previous element if that is possible.
func (iter *Iterator) Previous() (ok bool) {
	if iter.didSeek {
		panic("not implemented")
	}
	return iter.inner.Previous()
}

// Key returns the current key.
func (iter *Iterator) Key() interface{} {
	return iter.inner.Key()
}

// Value returns the current value.
func (iter *Iterator) Value() interface{} {
	return iter.inner.Value()
}

// Seek reduces iterative seek costs for searching forward into the Skip List
// by remarking the range of keys over which it has scanned before.  If the
// requested key occurs prior to the point, the Skip List will start searching
// as a safeguard.  It returns true if the key is within the known range of
// the list.
func (iter *Iterator) Seek(key interface{}) (ok bool) {
	iter.didSeek = true
	ok = iter.inner.Seek(key)
	iter.seekWasOK = ok
	return ok
}

// Close this iterator to reap resources associated with it.  While not
// strictly required, it will provide extra hints for the garbage collector.
func (iter *Iterator) Close() {
	iter.inner.Close()
}
