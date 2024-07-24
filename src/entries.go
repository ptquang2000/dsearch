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
	next   *Entry
}
type Entries struct {
	sigEntry    chan *Entry
	storage     map[string]*Entry
	list        []*Entry
	head        *Entry
	fzfDelegate FzfDelegate
	wg          sync.WaitGroup
	state       atomic.Int32
	count       atomic.Int32
}
type FilteredState struct {
	head         *Entry
	iter         *Entry
	count        int
	sigRefreshed chan RefreshedMsg
}
type State int32

const (
	Loading   State = 0
	Loaded    State = 1
	Filtering State = 2
	Filtered  State = 3
	Stopped   State = 4
)

///////////////////////////////////////////////////////////////////////////////

func NewEntries() Entries {
	return Entries{
		storage:     make(map[string]*Entry),
		fzfDelegate: NewFzfDelegate(),
	}
}

///////////////////////////////////////////////////////////////////////////////

func (p *Entries) LoadEntries(sigRefreshed chan RefreshedMsg) tea.Cmd {
	return func() tea.Msg {
		p.state.Store(int32(Loading))
		defer p.state.Store(int32(Loaded))

		p.sigEntry = make(chan *Entry)
		go p.appendEntry(sigRefreshed)
		loadApplications(p.sigEntry)
		loadFiles(p.sigEntry, true)
		close(p.sigEntry)
		return EntriesLoadedMsg{head: p.head, count: int(p.count.Load())}
	}
}

///////////////////////////////////////////////////////////////////////////////

func (p *Entries) appendEntry(sig chan RefreshedMsg) {
	entry := <-p.sigEntry
	p.list = append(p.list, entry)
	p.storage[entry.name] = entry
	p.head = entry
	p.count.Store(int32(len(p.list)))

	sig <- RefreshedMsg{head: p.head, count: int(p.count.Load())}
	for entry := range p.sigEntry {
		p.list[len(p.list)-1].next = entry
		p.list = append(p.list, entry)
		p.storage[entry.name] = entry
		p.count.Store(int32(len(p.list)))
		select {
		case sig <- RefreshedMsg{head: p.head, count: int(p.count.Load())}:
		default:
		}
	}
}

///////////////////////////////////////////////////////////////////////////////

func (p *Entries) FilterEntry(sigRefreshed chan RefreshedMsg, query string) tea.Cmd {
	return func() tea.Msg {
		if p.state.CompareAndSwap(int32(Filtering), int32(Stopped)) {
			p.wg.Wait()
		} else if p.state.CompareAndSwap(int32(Filtering), int32(Filtered)) {
			p.wg.Wait()
		}
		p.wg.Add(1)
		defer p.wg.Done()
		p.state.Store(int32(Filtering))
		defer p.state.Store(int32(Filtered))

		state := FilteredState{count: 0, sigRefreshed: sigRefreshed}
		filteredFn := func(name string) {
			p.processFilteredEntry(&state, name)
		}
		loopFn := func(input chan string) {
			p.loopEntries(input, len(p.storage))
		}
		p.fzfDelegate.execute(query, filteredFn, loopFn)

		if p.state.Load() != int32(Stopped) {
			return FilterFinMsg{head: state.head, count: state.count}
		}
		return FilterStopMsg{}
	}
}

///////////////////////////////////////////////////////////////////////////////

func (p *Entries) Stop() bool {
	if p.state.CompareAndSwap(int32(Filtering), int32(Stopped)) {
		return true
	}
	return false
}

///////////////////////////////////////////////////////////////////////////////

func (p *Entries) loopEntries(input chan string, count int) {
	for i := 0; i < count; i++ {
		if p.state.Load() == int32(Stopped) {
			break
		}
		input <- p.list[i].name
	}
	close(input)
}

///////////////////////////////////////////////////////////////////////////////

func (p *Entries) processFilteredEntry(state *FilteredState, name string) {
	if state.head == nil {
		state.head = p.storage[name]
		state.head.next = nil
		state.iter = state.head
		state.count = 1
	} else {
		state.iter.next = p.storage[name]
		state.iter = state.iter.next
		state.iter.next = nil
		state.count += 1
	}
	msg := RefreshedMsg{head: state.head, count: state.count}
	select {
	case state.sigRefreshed <- msg:
	default:
	}
}

///////////////////////////////////////////////////////////////////////////////
