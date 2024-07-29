package dsearch

import (
	"sync"
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"
)

///////////////////////////////////////////////////////////////////////////////

type IEntryManager interface {
	LoadEntries(...func(chan *Entry)) tea.Cmd
	FilterEntry(string) tea.Cmd
	StopFilter() bool
}

type EntryManager struct {
	storage     IEntryHashTable
	viewList    IEntryLinkedList
	fzfDelegate FzfDelegate
	dataReady   bool
	cond        sync.Cond
	wg          sync.WaitGroup
	state       atomic.Int32
	sigRefresh  SigRefresh
}

const (
	Stopped   int32 = 0
	Filtering int32 = 1
	Filtered  int32 = 2
)

///////////////////////////////////////////////////////////////////////////////

func NewEntryManager(signal SigRefresh) IEntryManager {
	return &EntryManager{
		storage:    NewEntryHashTable(),
		sigRefresh: signal,
	}
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryManager) LoadEntries(loaders ...func(chan *Entry)) tea.Cmd {
	return func() tea.Msg {
		entryChan := make(chan *Entry)
		defer close(entryChan)

		var mutex sync.Mutex
		p.cond = *sync.NewCond(&mutex)
		go p.appendEntry(entryChan)
		for _, loader := range loaders {
			loader(entryChan)
		}

		return LoadedMsg{}
	}
}

///////////////////////////////////////////////////////////////////////////////

func emit(signal SigRefresh, l IEntryLinkedList) {
	select {
	case signal <- RefreshingMsg{l}:
	default:
	}
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryManager) appendEntry(entryChan chan *Entry) {
	if p.viewList != nil {
		return
	}

	p.viewList = NewEntryLinkedList()
	for entry := range entryChan {
		p.storage.emplace(entry)
		p.viewList.append(entry)
		if p.state.Load() == Filtering {
			continue
		}
		emit(p.sigRefresh, p.viewList)
	}

	p.cond.L.Lock()
	p.dataReady = true
	p.cond.Broadcast()
	p.cond.L.Unlock()
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryManager) StopFilter() bool {
	var isFiltering bool
	p.cond.L.Lock()
	isFiltering = p.state.CompareAndSwap(Filtering, Stopped)
	p.cond.Broadcast()
	p.cond.L.Unlock()
	return isFiltering
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryManager) FilterEntry(query string) tea.Cmd {
	return func() tea.Msg {
		if !p.state.CompareAndSwap(Filtered, Filtering) ||
			!p.state.CompareAndSwap(Stopped, Filtering) {
			p.wg.Wait()
			p.state.Store(Filtering)
		}
		p.wg.Add(1)
		defer p.wg.Done()

		l := NewEntryLinkedList()
		if entry := loadCalculator(query); entry != nil {
			l.prepend(entry)
			emit(p.sigRefresh, l)
			query = entry.name
		}

		filterFn := func(name string) {
			entries := p.storage.get(name)
			l.appendEntries(entries)
			emit(p.sigRefresh, l)
		}
		p.fzfDelegate.Execute(query, filterFn, p.loopEntries)

		if p.state.CompareAndSwap(Filtering, Filtered) {
			return FilteredMsg{l}
		}
		return StoppedMsg{}
	}
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryManager) loopEntries(stream FzfStream) {
	iter := p.viewList.begin()
	count := p.viewList.len()
	condition := func() bool {
		return count != p.viewList.len() || p.dataReady || p.state.Load() == Stopped
	}
	for i := 0; i < count; i++ {
		stream <- iter.value()

		p.cond.L.Lock()
		for !condition() {
			p.cond.Wait()
		}
		p.cond.L.Unlock()

		if p.state.Load() == Stopped {
			break
		}

		count = p.viewList.len()
		iter = iter.next
	}
}

///////////////////////////////////////////////////////////////////////////////
