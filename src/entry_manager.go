package dsearch

import (
	"sync"

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
	mutex       sync.Mutex
	cond        sync.Cond
	dataReady   bool
	dataPending bool
	state       FilterState
	sigRefresh  SigRefresh
}

type FilterState int32

const (
	Stopped   FilterState = 0
	Stopping  FilterState = 1
	Filtering FilterState = 2
	Filtered  FilterState = 3
)

///////////////////////////////////////////////////////////////////////////////

func NewEntryManager(signal SigRefresh) IEntryManager {
	p := &EntryManager{
		storage:    NewEntryHashTable(),
		sigRefresh: signal,
	}
	p.cond = *sync.NewCond(&p.mutex)
	return p
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryManager) LoadEntries(loaders ...func(chan *Entry)) tea.Cmd {
	return func() tea.Msg {
		entryChan := make(chan *Entry)
		defer close(entryChan)

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

	shouldEmit := func() bool {
		p.cond.L.Lock()
		defer p.cond.Broadcast()
		defer p.cond.L.Unlock()

		p.dataPending = true
		return p.state != Filtering
	}

	p.viewList = NewEntryLinkedList()
	for entry := range entryChan {
		p.storage.emplace(entry)
		p.viewList.append(entry)
		if shouldEmit() {
			emit(p.sigRefresh, p.viewList)
		}
	}

	p.cond.L.Lock()
	p.dataReady = true
	p.cond.Broadcast()
	p.cond.L.Unlock()
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryManager) StopFilter() bool {
	p.cond.L.Lock()
	defer p.cond.Broadcast()
	defer p.cond.L.Unlock()
	if p.state == Filtering {
		p.state = Stopping
		return true
	}
	return false
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryManager) FilterEntry(query string) tea.Cmd {
	wait := func() {
		p.cond.L.Lock()
		defer p.cond.L.Unlock()
		for p.state != Filtered && p.state != Stopped {
			p.cond.Wait()
		}
		p.state = Filtering
	}
	return func() tea.Msg {
		wait()
		if len(query) == 0 {
			p.cond.L.Lock()
			defer p.cond.L.Unlock()
			p.state = Filtered
			return FilteredMsg{p.viewList}
		}

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

		p.cond.L.Lock()
		defer p.cond.L.Unlock()
		if p.state == Filtering {
			p.state = Filtered
			return FilteredMsg{l}
		}
		p.state = Stopped
		return StoppedMsg{l}
	}
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryManager) loopEntries(stream FzfStream) {
	wait := func() bool {
		p.cond.L.Lock()
		defer p.cond.L.Unlock()

		isStopped := p.state == Filtering
		for isStopped && !p.dataPending && !p.dataReady {
			p.cond.Wait()
			isStopped = p.state == Filtering
		}
		if p.dataPending {
			p.dataPending = false
		}
		return isStopped
	}
	for iter := p.viewList.begin(); iter != nil; iter = iter.next {
		stream <- iter.value()
		if !wait() {
			break
		}
	}
}

///////////////////////////////////////////////////////////////////////////////
