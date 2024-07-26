package dsearch

import (
	"crypto/sha256"
	"encoding/binary"
	"log"
	"sync"
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"
)

///////////////////////////////////////////////////////////////////////////////

type Entry struct {
	name   string
	action func()

	next  *Entry // next entry
	fnext *Entry // next filtered entry
}
type EntryHashMapTable map[uint64][]*Entry
type Entries struct {
	storage EntryHashMapTable
	head    *Entry
	count   atomic.Int32

	entryChan   chan *Entry
	fzfDelegate FzfDelegate

	mutex sync.Mutex
	wg    sync.WaitGroup
	state atomic.Int32
}
type EntryLinkedList struct {
	head   *Entry
	iter   *Entry
	count  int32
	signal SigRefresh
}

const (
	Stopped   int32 = 0
	Filtering int32 = 1
	Filtered  int32 = 2
)

///////////////////////////////////////////////////////////////////////////////

func NewEntries() Entries {
	return Entries{
		storage:     make(EntryHashMapTable),
		fzfDelegate: NewFzfDelegate(),
	}
}

///////////////////////////////////////////////////////////////////////////////

func (p *Entries) LoadEntries(signal SigRefresh) tea.Cmd {
	return func() tea.Msg {
		p.entryChan = make(chan *Entry)
		defer close(p.entryChan)

		go p.appendEntry(signal)
		loadApplications(p.entryChan)
		loadFiles(p.entryChan, true)

		return LoadedMsg{}
	}
}

///////////////////////////////////////////////////////////////////////////////

func hasher() func(string) uint64 {
	h := sha256.New()
	return func(s string) uint64 {
		h.Reset()
		h.Write([]byte(s))
		bs := h.Sum(nil)
		return binary.LittleEndian.Uint64(bs)
	}
}

///////////////////////////////////////////////////////////////////////////////

func (p *Entries) appendEntry(signal SigRefresh) {
	entry := <-p.entryChan

	hash := hasher()
	emplace := func(e *Entry) *Entry {
		p.mutex.Lock()
		defer p.mutex.Unlock()

		key := hash(e.name)
		if table, ok := p.storage[key]; ok {
			table = append(table, e)
			return table[len(table)-1]
		}
		p.storage[key] = make([]*Entry, 0, 1)
		p.storage[key] = append(p.storage[key], e)
		return e
	}

	p.head = emplace(entry)
	iter := p.head
	p.send(signal, RefreshingMsg{
		head:  p.head,
		count: p.count.Add(1),
	})

	for entry := range p.entryChan {
		iter.next = emplace(entry)
		iter.fnext = iter.next
		iter = iter.next
		p.send(signal, RefreshingMsg{
			head:  p.head,
			count: p.count.Add(1),
		})
	}
}

///////////////////////////////////////////////////////////////////////////////

func (p *Entries) send(signal SigRefresh, msg RefreshingMsg) {
	if p.state.Load() == Filtering {
		return
	}
	select {
	case signal <- msg:
	default:
	}
}

///////////////////////////////////////////////////////////////////////////////

func (p *Entries) StopFilter() bool {
	return p.state.CompareAndSwap(Filtering, Stopped)
}

///////////////////////////////////////////////////////////////////////////////

func (p *Entries) FilterEntry(signal SigRefresh, query string) tea.Cmd {
	return func() tea.Msg {
		if !p.state.CompareAndSwap(Filtered, Filtering) ||
			!p.state.CompareAndSwap(Stopped, Filtering) {
			p.wg.Wait()
			p.state.Store(Filtering)
		}
		p.wg.Add(1)
		defer p.wg.Done()

		l := EntryLinkedList{count: 0, signal: signal}
		hash := hasher()
		filterFn := func(name string) {
			p.processFilteredEntry(&l, hash(name))
		}
		loopFn := func(stream FzfStream) {
			p.loopEntries(stream)
		}
		p.fzfDelegate.execute(query, filterFn, loopFn)

		if p.state.CompareAndSwap(Filtering, Filtered) {
			return FilteredMsg{head: l.head, count: l.count}
		}
		return StoppedMsg{}
	}
}

///////////////////////////////////////////////////////////////////////////////

func (p *Entries) loopEntries(stream FzfStream) {
	iter := p.head
	i := int32(0)
	for i < p.count.Load() && p.state.Load() != Stopped && iter != nil {
		stream <- iter.name
		iter = iter.next
		i++
	}
	close(stream)
}

///////////////////////////////////////////////////////////////////////////////

func (p *Entries) processFilteredEntry(l *EntryLinkedList, key uint64) {
	get := func(key uint64) (*Entry, *Entry, int32) {
		p.mutex.Lock()
		defer p.mutex.Unlock()

		table, ok := p.storage[key]
		if !ok {
			log.Fatalf("Key %d not found in storage", key)
		}
		iter := table[0]
		for i := 1; i < len(table); i++ {
			iter.fnext = table[i]
			iter = iter.fnext
		}
		return table[0], table[len(table)-1], int32(len(table))
	}
	if l.head == nil {
		l.head, l.iter, l.count = get(key)
	} else {
		var count int32
		l.iter.fnext, l.iter, count = get(key)
		l.count += count
		if l.iter != nil {
			l.iter.fnext = nil
		}
	}
	msg := RefreshingMsg{head: l.head, count: l.count}
	select {
	case l.signal <- msg:
	default:
	}
}

///////////////////////////////////////////////////////////////////////////////

func (p *Entries) SelectEntry(entry *Entry) bool {
	if entry.action != nil {
		entry.action()
		return true
	}
	return false
}

///////////////////////////////////////////////////////////////////////////////
