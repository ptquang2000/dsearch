package dsearch

import (
	"math/rand"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
)

///////////////////////////////////////////////////////////////////////////////

func BenchmarkLoadEntries(b *testing.B) {
	// NOTE: Records
	// commit 6e1973f00fccdfc2da12ab91a0320d32d04f3657
	// BenchmarkFilterEntry-16		100         966271470 ns/op
	// commit 7939b70c8b2e40f2b69a9b4593c1e7c4d2f34385
	// BenchmarkLoadEntries-16		100         950366550 ns/op
	// commit 92d80fc1194da079556b44c0e62a0939ba195231
	// BenchmarkLoadEntries-16              100         918217414 ns/op
	for i := 0; i < b.N; i++ {
		m := NewEntryManager(nil, FzfConfig{true, true, 0})
		m.LoadEntries(
			func(c chan *Entry) { loadApplications(c) },
			func(c chan *Entry) { loadFiles(c, true) },
		)
	}
}

///////////////////////////////////////////////////////////////////////////////

func BenchmarkFilterEntry(b *testing.B) {
	// NOTE: Records
	// commit 6e1973f00fccdfc2da12ab91a0320d32d04f3657
	// BenchmarkFilterEntry-16              100         518055988 ns/op
	// commit 7939b70c8b2e40f2b69a9b4593c1e7c4d2f34385
	// BenchmarkFilterEntry-16              100         445740950 ns/op
	// commit 92d80fc1194da079556b44c0e62a0939ba195231
	// BenchmarkFilterEntry-16              100         479693914 ns/op
	// commit c3460cc6a60e1d9dcbaf5873ef0efa0a21bf5562
	// BenchmarkFilterEntry-16              100         444117061 ns/op
	// commit 2bc250b3d1239c8fcc85b4a046b39056cf3f884e
	// BenchmarkFilterEntry-16              100         256185131 ns/op

	m := NewEntryManager(nil, FzfConfig{true, true, 0})
	m.LoadEntries(func(entryChan chan *Entry) {
		for i := uint64(0); i < 1000000; i++ {
			entryChan <- &Entry{
				name: strconv.FormatUint(i, 10) + "_"}
		}
	})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.FilterEntry("999999_")
	}
}

///////////////////////////////////////////////////////////////////////////////

func extract(nodes []EntryNode) []string {
	var result []string
	for _, node := range nodes {
		result = append(result, node.Value())
	}
	return result
}

///////////////////////////////////////////////////////////////////////////////

func TestFilterEntry(t *testing.T) {
	var expected, result []string
	m := NewEntryManager(nil, FzfConfig{true, true, 0})
	loadDummies := func(entryChan chan *Entry) {
		for i := uint64(0); i < 1000000; i++ {
			entryChan <- &Entry{
				name: strconv.FormatUint(i, 10) + "_"}
		}
	}
	m.LoadEntries(loadDummies)

	result = extract(m.FilterEntry("42069_"))
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

	result = extract(m.FilterEntry("999999_"))
	expected = []string{"999999_"}
	if !slices.Equal(expected, result) {
		t.Errorf(`Expected %v got %v`, expected, result)
	}

	result = extract(m.FilterEntry("xxxxxx_"))
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
	m := NewEntryManager(nil, FzfConfig{true, true, 0})
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
	go m.LoadEntries(loadDummies)

	wg.Wait()
	result = extract(m.FilterEntry("69420_"))
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
	m := NewEntryManager(nil, FzfConfig{true, true, 0})
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
	go m.LoadEntries(loadDummies)

	wg.Wait()
	result = extract(m.FilterEntry("42069_"))
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
	m := NewEntryManager(nil, FzfConfig{true, true, 0})
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
	go m.LoadEntries(loadDummies)

	wg.Wait()
	result = extract(m.FilterEntry("6969_"))
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
	m := NewEntryManager(refreshCon, FzfConfig{true, true, 0})
	loadDummies := func(entryChan chan *Entry) {
		for i := uint64(0); i < 100000; i++ {
			entryChan <- &Entry{
				name: strconv.FormatUint(i, 10) + "_"}
		}
	}
	m.LoadEntries(loadDummies)

	query := "69_"
	length := len(m.FilterEntry(query))

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
	for i := 0; i < 10; i++ {
		go stopFilter()
		m.FilterEntry(query)
	}

	m.FilterEntry(query)
	close(refreshCon)
	fin.Wait()
}

///////////////////////////////////////////////////////////////////////////////

func TestSynchronizeFilterEntry(t *testing.T) {
	var fin sync.WaitGroup
	m := NewEntryManager(nil, FzfConfig{true, true, 0})
	loadDummies := func(entryChan chan *Entry) {
		for i := uint64(0); i < 100000; i++ {
			entryChan <- &Entry{
				name: strconv.FormatUint(i, 10) + "_"}
		}
	}
	m.LoadEntries(loadDummies)

	query := "420_"
	length := len(m.FilterEntry(query))
	times := 10

	result := 0
	filter := func() {
		result += len(m.FilterEntry(query))
		fin.Done()
	}
	for i := 0; i < times; i++ {
		fin.Add(1)
		go filter()
	}

	fin.Wait()
	expected := length * times
	if expected != result {
		t.Errorf(`Expected %d got %d`, expected, result)
	}
}

///////////////////////////////////////////////////////////////////////////////

func TestSynchronizeStopThenFilter(t *testing.T) {
	var fin sync.WaitGroup
	refreshCon := make(SigRefresh)
	m := NewEntryManager(refreshCon, FzfConfig{true, true, 0})
	loadDummies := func(entryChan chan *Entry) {
		for i := uint64(0); i < 1000000; i++ {
			entryChan <- &Entry{
				name: strconv.FormatUint(i, 10) + "_"}
		}
	}
	m.LoadEntries(loadDummies)

	var nums atomic.Int32
	query, result := "420_", 0
	expected := len(m.FilterEntry(query))
	half := expected / 2
	nums.Store(10)
	stopThenFilter := func() {
		nums.Add(-1)
		if nums.Load() == 1 && result != expected {
			t.Errorf(`Expected %d got %d`, expected, result)
			return
		}
		m.StopFilter()
		result = len(m.FilterEntry(query))
		if result < half {
			t.Errorf(`Expected >=%d got %d`, half, result)
		}
	}
	go func() {
		defer fin.Done()
		fin.Add(1)
		count := 0
		for range refreshCon {
			count += 1
			if nums.Load() > 1 && count >= rand.Intn(half) {
				count = 0
				go stopThenFilter()
			}
		}
	}()
	go stopThenFilter()

	close(refreshCon)
	fin.Wait()
}

///////////////////////////////////////////////////////////////////////////////
