package dsearch

import (
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
type Entries struct {
	storage map[string]*Entry
	head    *Entry
	count   atomic.Int32

	entryChan   chan *Entry
	fzfDelegate FzfDelegate

	mutex sync.Mutex
	wg    sync.WaitGroup
	state atomic.Int32
}
type State struct {
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
		storage:     make(map[string]*Entry),
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

func (p *Entries) appendEntry(signal SigRefresh) {
	entry := <-p.entryChan

	p.mutex.Lock()
	p.head = entry
	p.storage[entry.name] = p.head
	p.mutex.Unlock()

	p.send(signal, RefreshingMsg{
		head:  p.head,
		count: p.count.Add(1),
	})

	iter := p.head
	for entry := range p.entryChan {
		p.mutex.Lock()
		iter.next = entry
		iter.fnext = entry
		p.storage[iter.name] = iter
		iter = iter.next
		p.mutex.Unlock()

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

		s := State{count: 0, signal: signal}
		filterFn := func(name string) {
			p.processFilteredEntry(&s, name)
		}
		loopFn := func(stream FzfStream) {
			p.loopEntries(stream, p.count.Load())
		}
		p.fzfDelegate.execute(query, filterFn, loopFn)

		if p.state.CompareAndSwap(Filtering, Filtered) {
			return FilteredMsg{head: s.head, count: s.count}
		}
		return StoppedMsg{}
	}
}

///////////////////////////////////////////////////////////////////////////////

func (p *Entries) loopEntries(stream FzfStream, count int32) {
	iter := p.head
	for i := int32(0); i < count && iter != nil; i++ {
		if p.state.Load() == Stopped {
			break
		}
		stream <- iter.name
		iter = iter.next
	}
	close(stream)
}

///////////////////////////////////////////////////////////////////////////////

func (p *Entries) processFilteredEntry(state *State, name string) {
	if state.head == nil {
		p.mutex.Lock()
		state.head = p.storage[name]
		p.mutex.Unlock()

		state.head.fnext = nil
		state.iter = state.head
		state.count = 1
	} else {
		p.mutex.Lock()
		state.iter.fnext = p.storage[name]
		p.mutex.Unlock()

		state.iter = state.iter.fnext
		if state.iter != nil {
			state.iter.fnext = nil
		}
		state.count += 1
	}
	msg := RefreshingMsg{head: state.head, count: state.count}
	select {
	case state.signal <- msg:
	default:
	}
}

///////////////////////////////////////////////////////////////////////////////
