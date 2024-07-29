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

func emit(signal SigRefresh, entries []EntryNode) {
	select {
	case signal <- RefreshedMsg{entries}:
	default:
	}
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryManager) appendEntry(entryChan chan *Entry) {
	shouldEmit := func() bool {
		p.cond.L.Lock()
		defer p.cond.Broadcast()
		defer p.cond.L.Unlock()

		p.dataPending = true
		return p.state != Filtering
	}

	nodes := []EntryNode{}
	for entry := range entryChan {
		p.storage.emplace(entry)
		if shouldEmit() {
			nodes = append(nodes, entry)
			emit(p.sigRefresh, nodes)
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
	var nodes []EntryNode
	filterFn := func(name string) {
		for _, e := range p.storage.get(name) {
			nodes = append(nodes, e)
		}
		emit(p.sigRefresh, nodes)
	}
	return func() tea.Msg {
		wait()

		nodes = nil
		if entry := loadCalculator(query); entry != nil {
			nodes = append(nodes, entry)
			emit(p.sigRefresh, nodes)
			query = entry.name
		}
		p.fzfDelegate.Execute(query, filterFn, p.loopEntries)

		p.cond.L.Lock()
		defer p.cond.L.Unlock()
		if p.state == Filtering {
			p.state = Filtered
			return FilteredMsg{nodes}
		}
		p.state = Stopped
		return StoppedMsg{nodes}
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
	for iter := p.storage.begin(); iter != nil; iter = iter.Next() {
		stream <- iter.Value()
		if !wait() {
			break
		}
	}
}

///////////////////////////////////////////////////////////////////////////////
