package dsearch

import (
	"slices"
	"strconv"
	"sync"
	"testing"
)

///////////////////////////////////////////////////////////////////////////////

func TestEntryHashTable(t *testing.T) {
	table := EntryHashTable{
		hash:  hash(),
		table: make(map[uint32][]int),
	}
	var p IEntryHashTable = &table
	var result, expected []string

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

	if len(table.table) != 10 {
		t.Errorf(`0: Expected len %d got %d`, 10, len(table.table))
	}

	expected = []string{"3"}
	result = extract(p.transform(expected))
	slices.Sort(expected)
	if slices.Compare(expected, result) != 0 {
		t.Errorf(`Expected %v got %v`, expected, result)
	}

	expected = []string{"5"}
	result = extract(p.transform(expected))
	slices.Sort(expected)
	if slices.Compare(expected, result) != 0 {
		t.Errorf(`Expected %v got %v`, expected, result)
	}

	expected = []string{"1", "6", "9"}
	result = extract(p.transform(expected))
	slices.Sort(expected)
	if slices.Compare(expected, result) != 0 {
		t.Errorf(`Expected %v got %v`, expected, result)
	}
}

///////////////////////////////////////////////////////////////////////////////

func TestEntryForEachOrder(t *testing.T) {
	var p IEntryHashTable = NewEntryHashTable()
	var wg sync.WaitGroup
	var result, expected []string
	fn := func(start, end int64) {
		for i := start; i < end; i++ {
			p.emplace(&Entry{name: strconv.FormatInt(i, 10)})
		}
		wg.Done()
	}
	wg.Add(3)
	fn(6, 10)
	fn(3, 6)
	fn(0, 3)
	wg.Wait()

	p.forEach(3, 6, func(s string) bool {
		result = append(result, s)
		return true
	})

	expected = []string{"9", "3", "4"}
	if slices.Compare(expected, result) != 0 {
		t.Errorf(`Expected %v got %v`, expected, result)
	}
}

///////////////////////////////////////////////////////////////////////////////

func TestEntryTraverseOrder(t *testing.T) {
	var p IEntryHashTable = NewEntryHashTable()
	var wg sync.WaitGroup
	var result, expected []string
	fn := func(start, end int64) {
		for i := start; i < end; i++ {
			p.emplace(&Entry{name: strconv.FormatInt(i, 10)})
		}
		wg.Done()
	}
	wg.Add(3)
	fn(6, 10)
	fn(3, 6)
	fn(0, 3)
	wg.Wait()

	p.traverse(6, func(s string) bool {
		result = append(result, s)
		return true
	})

	expected = []string{"5", "0", "1", "2"}
	if slices.Compare(expected, result) != 0 {
		t.Errorf(`Expected %v got %v`, expected, result)
	}
}

///////////////////////////////////////////////////////////////////////////////

func TestEntryForEach(t *testing.T) {
	var p IEntryHashTable = NewEntryHashTable()
	var wg sync.WaitGroup
	var result, expected []string
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

	p.forEach(0, 10, func(s string) bool {
		result = append(result, s)
		return true
	})

	expected = []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}
	slices.Sort(result)
	if slices.Compare(expected, result) != 0 {
		t.Errorf(`Expected %v got %v`, expected, result)
	}
}

///////////////////////////////////////////////////////////////////////////////

func TestEntryTraverse(t *testing.T) {
	var p IEntryHashTable = NewEntryHashTable()
	var wg sync.WaitGroup
	var result, expected []string
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

	p.traverse(0, func(s string) bool {
		result = append(result, s)
		return true
	})

	expected = []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}
	slices.Sort(result)
	if slices.Compare(expected, result) != 0 {
		t.Errorf(`Expected %v got %v`, expected, result)
	}
}

///////////////////////////////////////////////////////////////////////////////
