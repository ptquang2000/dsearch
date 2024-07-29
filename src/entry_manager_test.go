package dsearch

import (
	"math/rand"
	"slices"
	"strconv"
	"sync"
	"testing"
)

///////////////////////////////////////////////////////////////////////////////

func BenchmarkLoadEntries(b *testing.B) {
	// NOTE: Records
	// commits 6e1973f00fccdfc2da12ab91a0320d32d04f3657
	// BenchmarkFilterEntry-16		100         966271470 ns/op
	for i := 0; i < b.N; i++ {
		m := NewEntryManager(nil)
		m.LoadEntries(
			func(c chan *Entry) { loadApplications(c) },
			func(c chan *Entry) { loadFiles(c, true) },
		)()
	}
}

///////////////////////////////////////////////////////////////////////////////

func BenchmarkFilterEntry(b *testing.B) {
	// NOTE: Records
	// commits 6e1973f00fccdfc2da12ab91a0320d32d04f3657
	// BenchmarkFilterEntry-16              100         518055988 ns/op
	m := NewEntryManager(nil)
	m.LoadEntries(func(entryChan chan *Entry) {
		for i := uint64(0); i < 1000000; i++ {
			entryChan <- &Entry{
				name: strconv.FormatUint(i, 10) + "_"}
		}
	})()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.FilterEntry("999999_")()
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
	// NOTE:
	// Begin to filter while loading all entries.
	// The result contains only loaded entries after starting point.
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
	// NOTE:
	// Begin to filter while loading all entries.
	// The result contains only loaded entries before starting point.
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
	// NOTE:
	// Begin to filter while loading all entries.
	// The result contains loaded entries before and after starting point.
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

func TestStopFilter(t *testing.T) {
	var fin sync.WaitGroup
	refreshCon := make(SigRefresh)
	m := NewEntryManager(refreshCon)
	loadDummies := func(entryChan chan *Entry) {
		for i := uint64(0); i < 100000; i++ {
			entryChan <- &Entry{
				name: strconv.FormatUint(i, 10) + "_"}
		}
	}
	m.LoadEntries(loadDummies)()

	expectFin := func(s string) int {
		if msg, ok := m.FilterEntry(s)().(FilteredMsg); !ok {
			t.Errorf(`Expected FilteredMsg got %v`, msg)
		} else {
			return msg.list.len()
		}
		return 0
	}
	query := "69_"
	length := expectFin(query)

	stopFilter := func() {
		fin.Add(1)
		defer fin.Done()

		count := 0
		for {
			_, ok := <-refreshCon
			if !ok {
				break
			}
			if count >= length {
				break
			}
			if count >= rand.Intn(length) {
				m.StopFilter()
				return
			}
			count += 1
		}
		t.Errorf(`Should not be stopped by closed chan`)
	}
	expectStop := func(s string) {
		if msg, ok := m.FilterEntry(s)().(StoppedMsg); !ok {
			t.Errorf(`Expected StoppedMsg got %v`, msg)
		}
	}
	for i := 0; i < 10; i++ {
		go stopFilter()
		expectStop(query)
	}

	expectFin(query)
	close(refreshCon)
	fin.Wait()
}
