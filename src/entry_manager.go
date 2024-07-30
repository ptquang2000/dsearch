package dsearch

import (
	"runtime"
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
	fzfDelegate IFzfDelegate
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

func NewEntryManager(signal SigRefresh, cfg FzfConfig) IEntryManager {
	p := &EntryManager{
		storage:     NewEntryHashTable(),
		fzfDelegate: NewFzfDelegate(cfg),
		sigRefresh:  signal,
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

func emit(signal SigRefresh, entries IEntryLinkedList) {
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

	for entry := range entryChan {
		p.storage.emplace_back(entry)
		if shouldEmit() {
			emit(p.sigRefresh, p.storage)
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
	var nodes IEntryLinkedList
	foundFn := func(name string) {
		for _, e := range p.storage.get(name) {
			nodes.push_back(e)
		}
		emit(p.sigRefresh, nodes)
	}
	return func() tea.Msg {
		wait()

		nodes = NewEntryLinkedList()

		if entry := loadCalculator(query); entry != nil {
			nodes.emplace_back(entry)
			emit(p.sigRefresh, nodes)
			query = entry.name
		}

		p.filterSync(foundFn, query)

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

func (p *EntryManager) filterSync(foundFn func(string), query string) {
	p.fzfDelegate.ExecuteSync(query, foundFn, p.readSync())
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryManager) readSync() func(FzfStream) {
	stopOrWait := func() bool {
		p.cond.L.Lock()
		defer p.cond.L.Unlock()

		isStopped := p.state != Filtering
		for isStopped && !(p.dataPending || p.dataReady) {
			p.cond.Wait()
			isStopped = p.state != Filtering
		}
		if p.dataPending {
			p.dataPending = false
		}
		return isStopped
	}
	return func(stream FzfStream) {
		it := p.storage.begin()
		for it != nil && !stopOrWait() {
			stream <- it.Value()
			it = it.Next()
		}
	}
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryManager) filterAsync(foundFn func(string), query string) {
	routines := runtime.NumCPU() - 1
	chunk := p.storage.len() / routines
	workers := []*sync.WaitGroup{}
	for i := range routines {
		var end EntryNode = nil
		if i < routines-1 {
			end = p.storage.at(i*chunk + chunk)
		}
		workers = append(
			workers,
			p.fzfDelegate.ExecuteAsync(
				query,
				foundFn,
				p.readAsync(p.storage.at(i*chunk), end)))
	}
	for _, worker := range workers {
		worker.Wait()
	}
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryManager) readAsync(start, end EntryNode) func(FzfStream) {
	stop := func() bool {
		p.cond.L.Lock()
		defer p.cond.L.Unlock()

		return p.state != Filtering
	}
	wait := func(skip bool) {
		if skip {
			return
		}
		p.cond.L.Lock()
		defer p.cond.L.Unlock()

		for !p.dataPending && !p.dataReady {
			p.cond.Wait()
		}
		if p.dataPending {
			p.dataPending = false
		}
	}
	return func(s FzfStream) {
		for it := start; it != end && !stop(); it = it.Next() {
			s <- it.Value()
			wait(end != nil)
		}
	}
}

///////////////////////////////////////////////////////////////////////////////
