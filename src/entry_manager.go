package dsearch

import (
	"runtime"
	"sync"
)

///////////////////////////////////////////////////////////////////////////////

type IEntryManager interface {
	LoadEntries(...func(chan *Entry))
	FilterEntry(string) []EntryNode
	StopFilter()
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
)

///////////////////////////////////////////////////////////////////////////////

func NewEntryManager(signal SigRefresh, cfg FzfConfig) IEntryManager {
	p := &EntryManager{
		storage:     NewEntryHashTable(),
		fzfDelegate: NewFzfDelegate(cfg),
		sigRefresh:  signal,
		state:       Stopped,
	}
	p.cond = *sync.NewCond(&p.mutex)
	return p
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryManager) LoadEntries(loaders ...func(chan *Entry)) {
	entryChan := make(chan *Entry)
	defer close(entryChan)

	go p.appendEntry(entryChan)
	for _, loader := range loaders {
		loader(entryChan)
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
		defer p.cond.L.Unlock()
		defer p.cond.Signal()

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

func (p *EntryManager) StopFilter() {
	p.cond.L.Lock()
	if p.state == Filtering {
		p.state = Stopping
		p.cond.Broadcast()
	}
	p.cond.L.Unlock()
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryManager) FilterEntry(query string) []EntryNode {
	defer func() {
		p.cond.L.Lock()
		p.state = Stopped
		p.cond.Signal()
		p.cond.L.Unlock()
	}()

	p.cond.L.Lock()
	for p.state != Stopped {
		p.cond.Wait()
	}
	p.state = Filtering
	p.cond.L.Unlock()

	if entry := loadCalculator(query); entry != nil {
		p.storage.emplace(entry)
		query = entry.name
	}

	return p.storage.transform(*p.filterAsync(query))
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryManager) filterSync(query string) *[]string {
	var strs []string
	foundFn := func(str string) {
		strs = append(strs, str)
		emit(p.sigRefresh, p.storage.transform(strs))
	}
	p.fzfDelegate.ExecuteSync(query, foundFn, p.readSync(0))
	return &strs
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryManager) filterAsync(query string) *[]string {
	var workers []*sync.WaitGroup
	var strs []string
	var mutex sync.Mutex

	defer func() {
		for _, worker := range workers {
			worker.Wait()
		}
	}()

	foundFn := func(str string) {
		mutex.Lock()
		defer mutex.Unlock()
		strs = append(strs, str)
		emit(p.sigRefresh, p.storage.transform(strs))
	}

	routines := runtime.NumCPU() - 1
	chunk := p.storage.len() / routines
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
	return &strs
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryManager) readSync(start int) func(FzfStream) {
	persist := func() bool {
		p.cond.L.Lock()
		defer p.cond.L.Unlock()

		filtering := p.state == Filtering
		hasData := p.dataPending || p.dataReady
		for filtering && !hasData {
			p.cond.Wait()
			filtering = p.state == Filtering
			hasData = p.dataPending || p.dataReady
		}
		if p.dataPending {
			p.dataPending = false
		}
		return filtering
	}
	return func(stream FzfStream) {
		p.storage.traverse(start, func(val string) bool {
			stream <- val
			return persist()
		})
	}
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryManager) readAsync(start, end int) func(FzfStream) {
	persist := func() bool {
		p.cond.L.Lock()
		defer p.cond.L.Unlock()
		return p.state == Filtering
	}
	return func(stream FzfStream) {
		p.storage.forEach(start, end, func(s string) bool {
			stream <- s
			return persist()
		})
	}
}

///////////////////////////////////////////////////////////////////////////////
