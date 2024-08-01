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

func emit(signal SigRefresh, nodes []EntryNode) {
	select {
	case signal <- RefreshedMsg{nodes}:
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
		p.storage.emplace(entry)
		if shouldEmit() {
			emit(p.sigRefresh, p.storage.getRawData())
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

		if entry := loadCalculator(query); entry != nil {
			p.storage.emplace(entry)
			query = entry.name
		}

		results := p.storage.transform(p.filterAsync(query))

		p.cond.L.Lock()
		defer p.cond.L.Unlock()
		if p.state == Filtering {
			p.state = Filtered
			return FilteredMsg{results}
		}
		p.state = Stopped
		return StoppedMsg{results}
	}
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryManager) filterSync(foundFn func(string), query string) {
	p.fzfDelegate.ExecuteSync(query, foundFn, p.readSync(0))
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryManager) readSync(start int) func(FzfStream) {
	isStopped := func() bool {
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
		p.storage.traverse(start, func(val string) bool {
			stream <- val
			return !isStopped()
		})
	}
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryManager) filterAsync(query string) []string {
	routines := runtime.NumCPU() - 1
	chunk := p.storage.len() / routines
	workers := []*sync.WaitGroup{}
	strs := []string{}
	var mutex sync.Mutex
	foundFn := func(str string) {
		mutex.Lock()
		defer mutex.Unlock()
		strs = append(strs, str)
		emit(p.sigRefresh, p.storage.transform(strs))
	}
	for i := range routines {
		var readFn func(FzfStream)
		if i < routines-1 {
			readFn = p.readAsync(i*chunk, i*chunk+chunk)
		} else {
			readFn = p.readSync(i * chunk)
		}
		workers = append(
			workers,
			p.fzfDelegate.ExecuteAsync(query, foundFn, readFn))
	}
	for _, worker := range workers {
		worker.Wait()
	}
	return strs
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryManager) readAsync(start, end int) func(FzfStream) {
	isStopped := func() bool {
		p.cond.L.Lock()
		defer p.cond.L.Unlock()

		return p.state != Filtering
	}
	return func(stream FzfStream) {
		p.storage.forEach(start, end, func(s string) bool {
			stream <- s
			return !isStopped()
		})
	}
}

///////////////////////////////////////////////////////////////////////////////
