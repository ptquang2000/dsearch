package dsearch

import (
	"sync"
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"
)

///////////////////////////////////////////////////////////////////////////////

type IEntryManager interface {
	LoadEntries(SigRefresh) tea.Cmd
	FilterEntry(SigRefresh, string) tea.Cmd
	StopFilter() bool
}

type EntryManager struct {
	storage     IEntryHashTable
	viewList    IEntryLinkedList
	fzfDelegate FzfDelegate
	wg          sync.WaitGroup
	state       atomic.Int32
}

const (
	Stopped   int32 = 0
	Filtering int32 = 1
	Filtered  int32 = 2
)

///////////////////////////////////////////////////////////////////////////////

func NewEntryManager() IEntryManager {
	return &EntryManager{
		storage:     NewEntryHashTable(),
		fzfDelegate: NewFzfDelegate(),
	}
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryManager) LoadEntries(signal SigRefresh) tea.Cmd {
	return func() tea.Msg {
		entryChan := make(chan *Entry)
		defer close(entryChan)

		go p.appendEntry(entryChan, signal)
		loadApplications(entryChan)
		loadFiles(entryChan, true)

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

func (p *EntryManager) appendEntry(entryChan chan *Entry, signal SigRefresh) {
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
		emit(signal, p.viewList)
	}
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryManager) StopFilter() bool {
	return p.state.CompareAndSwap(Filtering, Stopped)
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryManager) FilterEntry(signal SigRefresh, query string) tea.Cmd {
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
			emit(signal, l)
			query = entry.name
		}

		filterFn := func(name string) {
			entries := p.storage.get(name)
			l.appendEntries(entries)
			emit(signal, l)
		}
		loopFn := func(stream FzfStream) {
			p.loopEntries(stream)
		}
		p.fzfDelegate.execute(query, filterFn, loopFn)

		if p.state.CompareAndSwap(Filtering, Filtered) {
			return FilteredMsg{l}
		}
		return StoppedMsg{}
	}
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryManager) loopEntries(stream FzfStream) {
	iter := p.viewList.begin()
	for i := 0; i < p.viewList.len() && p.state.Load() != Stopped; i++ {
		stream <- iter.value()
		iter = iter.next
	}
	close(stream)
}

///////////////////////////////////////////////////////////////////////////////
