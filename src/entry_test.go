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
		storage: make(map[uint32][]EntryNode),
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
		if e.Value() != "3" {
			t.Errorf(`1: Expected %s got %s`, "3", e.Value())
		}
	}

	entries = p.get("5")
	if len(entries) != 2 {
		t.Errorf(`2: Expected len %d got %d`, 2, len(entries))
	}
	for _, e := range entries {
		if e.Value() != "5" {
			t.Errorf(`2: Expected %s got %s`, "5", e.Value())
		}
	}

	entries = p.get("9")
	if len(entries) != 3 {
		t.Errorf(`3: Expected len %d got %d`, 3, len(entries))
	}
	for _, e := range entries {
		if e.Value() != "9" {
			t.Errorf(`3: Expected %s got %s`, "9", e.Value())
		}
	}
}

func TestEntryTraverse(t *testing.T) {
	var p IEntryHashTable = NewEntryHashTable()
	var wg sync.WaitGroup
	fn := func(start, end int64) {
		for i := start; i < end; i++ {
			p.emplace(&Entry{name: strconv.FormatInt(i, 10)})
		}
		wg.Done()
	}
	wg.Add(3)
	go fn(6, 10)
	go fn(0, 3)
	go fn(3, 6)
	wg.Wait()

	var result []int64
	for it := p.begin(); it != nil; it = it.Next() {
		num, _ := strconv.ParseInt(it.Value(), 10, 64)
		result = append(result, num)
	}

	if len(result) != 10 {
		t.Errorf(`0: Expect %d got %d`, 10, len(result))
	}
	expected := []int64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	for _, num := range expected {
		if ok := slices.Contains(result, num); !ok {
			t.Errorf(`1: Value %d is not contained in %v`,
				num,
				result)
		}
	}
}
