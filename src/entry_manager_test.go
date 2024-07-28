package dsearch

import (
	"slices"
	"strconv"
	"sync"
	"testing"
)

///////////////////////////////////////////////////////////////////////////////

func BenchmarkLoadEntries(b *testing.B) {
	for i := 0; i < b.N; i++ {
		m := NewEntryManager(nil)
		m.LoadEntries(
			func(c chan *Entry) { loadApplications(c) },
			func(c chan *Entry) { loadFiles(c, true) },
		)()
	}
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryNode) extract() []string {
	var result []string
	for i := p; i != nil; i = i.next {
		result = append(result, i.value())
	}
	return result
}

///////////////////////////////////////////////////////////////////////////////

func TestFilterEntry(t *testing.T) {
	var expected, result []string
	m := NewEntryManager(nil)
	loadDummies := func(entryChan chan *Entry) {
		for i := uint64(0); i < 1000000; i++ {
			entryChan <- &Entry{
				name: strconv.FormatUint(i, 10) + "_"}
		}
	}
	m.LoadEntries(loadDummies)()
	filterEntry := func(s string) {
		if msg, ok := m.FilterEntry(s)().(FilteredMsg); ok {
			result = msg.list.begin().extract()
		} else {
			t.Errorf(`Expected FilteredMsg got %v`, msg)
		}
	}

	filterEntry("42069_")
	expected = []string{
		"42069_",
		"142069_",
		"242069_",
		"342069_",
		"442069_",
		"542069_",
		"642069_",
		"742069_",
		"842069_",
		"942069_"}
	if !slices.Equal(expected, result) {
		t.Errorf(`Expected %v got %v`, expected, result)
	}

	filterEntry("999999_")
	expected = []string{"999999_"}
	if !slices.Equal(expected, result) {
		t.Errorf(`Expected %v got %v`, expected, result)
	}

	filterEntry("xxxxxx_")
	expected = []string{}
	if !slices.Equal(expected, result) {
		t.Errorf(`Expected %v got %v`, expected, result)
	}
}

///////////////////////////////////////////////////////////////////////////////

func TestFilterEntryBeforeDataReadyCase1(t *testing.T) {
	var expected, result []string
	var wg, fin sync.WaitGroup
	m := NewEntryManager(nil)
	loadDummies := func(entryChan chan *Entry) {
		for i := uint64(0); i < 100000; i++ {
			entryChan <- &Entry{
				name: strconv.FormatUint(i, 10) + "_"}
			if i == 42690 {
				wg.Done()
			}
		}
		fin.Done()
	}
	fin.Add(1)
	wg.Add(1)
	go m.LoadEntries(loadDummies)()
	filterEntry := func(s string) {
		if msg, ok := m.FilterEntry(s)().(FilteredMsg); ok {
			result = msg.list.begin().extract()
		} else {
			t.Errorf(`Expected FilteredMsg got %v`, msg)
		}
	}

	wg.Wait()
	filterEntry("69420_")
	expected = []string{"69420_"}
	if !slices.Equal(expected, result) {
		t.Errorf(`Expected %v got %v`, expected, result)
	}
	fin.Wait()
}

///////////////////////////////////////////////////////////////////////////////

func TestFilterEntryBeforeDataReadyCase2(t *testing.T) {
	var expected, result []string
	var wg, fin sync.WaitGroup
	m := NewEntryManager(nil)
	loadDummies := func(entryChan chan *Entry) {
		for i := uint64(0); i < 100000; i++ {
			entryChan <- &Entry{
				name: strconv.FormatUint(i, 10) + "_"}
			if i == 69420 {
				wg.Done()
			}
		}
		fin.Done()
	}
	fin.Add(1)
	wg.Add(1)
	go m.LoadEntries(loadDummies)()
	filterEntry := func(s string) {
		if msg, ok := m.FilterEntry(s)().(FilteredMsg); ok {
			result = msg.list.begin().extract()
		} else {
			t.Errorf(`Expected FilteredMsg got %v`, msg)
		}
	}

	wg.Wait()
	filterEntry("42069_")
	expected = []string{"42069_"}
	if !slices.Equal(expected, result) {
		t.Errorf(`Expected %v got %v`, expected, result)
	}
	fin.Wait()
}

///////////////////////////////////////////////////////////////////////////////

func TestFilterEntryBeforeDataReadyCase3(t *testing.T) {
	var expected, result []string
	var wg, fin sync.WaitGroup
	m := NewEntryManager(nil)
	loadDummies := func(entryChan chan *Entry) {
		for i := uint64(0); i < 100000; i++ {
			entryChan <- &Entry{
				name: strconv.FormatUint(i, 10) + "_"}
			if i == 46969 {
				wg.Done()
			}
		}
		fin.Done()
	}
	fin.Add(1)
	wg.Add(1)
	go m.LoadEntries(loadDummies)()
	filterEntry := func(s string) {
		if msg, ok := m.FilterEntry(s)().(FilteredMsg); ok {
			result = msg.list.begin().extract()
		} else {
			t.Errorf(`Expected FilteredMsg got %v`, msg)
		}
	}

	wg.Wait()
	filterEntry("6969_")
	expected = []string{
		"6969_",
		"16969_",
		"26969_",
		"36969_",
		"46969_",
		"56969_",
		"66969_",
		"76969_",
		"86969_",
		"96969_",
	}
	if !slices.Equal(expected, result) {
		t.Errorf(`Expected %v got %v`, expected, result)
	}
	fin.Wait()
}

///////////////////////////////////////////////////////////////////////////////
