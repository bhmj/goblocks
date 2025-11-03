package memcache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCache(t *testing.T) {
	a := assert.New(t)

	recSize := RecordSize()
	mc := New(3 * recSize) // bytes

	// fill up cache
	mc.Set("a", 123)
	mc.Set("b", 456)
	mc.Set("c", 789)

	// check size
	sz := mc.Size()
	a.Equal(3*recSize, sz)

	// value in cache
	v, _, _ := mc.Get("a")
	a.Equal(123, v.(int))

	// stale "a" value
	raw := mc.(*cache)
	raw.stale("a")

	// add element, evicting "a"
	mc.Set("d", 987)

	// "d" is in cache
	_, _, found := mc.Get("d")
	a.True(found)

	// value is evicted
	v, _, found = mc.Get("a")
	a.False(found)
	a.Nil(v)

	// make "b" frequently used
	for i := 0; i < 100; i++ {
		mc.Get("b")
	}

	// allow all cache entries to be evicted
	raw.stale("b")
	raw.stale("c")
	raw.stale("d")

	// make new entries
	mc.Set("x", 654)
	mc.Set("y", 321)

	// Two entries were evicted to free up space for the two new entries;
	// However, b should not be evicted since is was used much recently.

	// "b" should stay
	v, _, _ = mc.Get("b")
	a.Equal(456, v.(int))
}
