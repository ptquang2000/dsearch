package dsearch

import (
	"slices"
	"strconv"
	"sync"
	"testing"
)

func TestEntryHashTable(t *testing.T) {
	table := EntryHashTable{
		hash:    hash(),
		storage: make(map[uint32][]*Entry),
	}
	var p IEntryHashTable = &table

	var wg sync.WaitGroup
	fn := func(start, end int64) {
		for i := start; i < end; i++ {
			p.emplace(&Entry{name: strconv.FormatInt(i, 10)})
		}
		wg.Done()
	}

	wg.Add(3)
	go fn(7, 10)
	go fn(5, 10)
	go fn(0, 10)
	wg.Wait()

	if len(table.storage) != 10 {
		t.Errorf(`0: Expected len %d got %d`, 10, len(table.storage))
	}

	entries := p.get("3")
	if len(entries) != 1 {
		t.Errorf(`1: Expected len %d got %d`, 1, len(entries))
	}
	for _, e := range entries {
		if e.name != "3" {
			t.Errorf(`1: Expected %s got %s`, "3", e.name)
		}
	}

	entries = p.get("5")
	if len(entries) != 2 {
		t.Errorf(`2: Expected len %d got %d`, 2, len(entries))
	}
	for _, e := range entries {
		if e.name != "5" {
			t.Errorf(`2: Expected %s got %s`, "5", e.name)
		}
	}

	entries = p.get("9")
	if len(entries) != 3 {
		t.Errorf(`3: Expected len %d got %d`, 3, len(entries))
	}
	for _, e := range entries {
		if e.name != "9" {
			t.Errorf(`3: Expected %s got %s`, "9", e.name)
		}
	}
}

func TestEntryLinkedList(t *testing.T) {
	list := &EntryLinkedList{}
	var p IEntryLinkedList = list
	var wg sync.WaitGroup
	fn := func(start, end int64) {
		for i := start; i < end; i++ {
			p.append(&Entry{name: strconv.FormatInt(i, 10)})
		}
		wg.Done()
	}
	wg.Add(3)
	go fn(6, 10)
	go fn(0, 3)
	go fn(3, 6)
	wg.Wait()

	if p.len() != 10 {
		t.Errorf(`0: Expect %d got %d`, 10, p.len())
	}
	expected := []int64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	iter := list.begin()
	var count int = 0
	for ; iter != nil; count++ {
		val, _ := strconv.ParseInt(iter.value(), 10, 64)
		if ok := slices.Contains(expected, val); !ok {
			t.Errorf(`1: Value %d is not contained in %v`,
				val,
				expected[count])
		}
		iter = iter.next
	}
	if count != 10 {
		t.Errorf(`0: Expect %d got %d`, 10, count)
	}
}
