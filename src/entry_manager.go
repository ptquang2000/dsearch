package dsearch

import (
	"runtime"
	"sync"
)

///////////////////////////////////////////////////////////////////////////////

type FilterState int32

const (
	Stopped   FilterState = 0
	Stopping  FilterState = 1
	Filtering FilterState = 2
)

///////////////////////////////////////////////////////////////////////////////

type IEntryManager interface {
	LoadEntries(SigLoad, ...func(SigEntry)) int
	FilterEntry(SigRefresh, string) []EntryNode
	StopFilter(bool)
}

///////////////////////////////////////////////////////////////////////////////

type EntryManager struct {
	storage     IEntryHashTable
	fzfDelegate IFzfDelegate
	mutex       sync.Mutex
	cond        sync.Cond
	dataReady   bool
	dataPending bool
	state       FilterState
}

///////////////////////////////////////////////////////////////////////////////

const k_chunkSize = 1000

///////////////////////////////////////////////////////////////////////////////

func NewEntryManager(cfg FzfConfig) IEntryManager {
	p := &EntryManager{
		storage:     NewEntryHashTable(),
		fzfDelegate: NewFzfDelegate(cfg),
		state:       Stopped,
	}
	p.cond = *sync.NewCond(&p.mutex)
	return p
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryManager) LoadEntries(s SigLoad, cbs ...func(SigEntry)) int {
	entryChan := make(SigEntry)
	defer close(entryChan)

	go p.appendEntry(s, entryChan)
	for _, loader := range cbs {
		loader(entryChan)
	}
	return p.storage.len()
}

///////////////////////////////////////////////////////////////////////////////

func emit[T any](signal chan T, param T) {
	select {
	case signal <- param:
	default:
	}
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryManager) appendEntry(s1 SigLoad, s2 SigEntry) {
	count := 0
	for entry := range s2 {
		p.storage.emplace(entry)

		p.cond.L.Lock()
		p.dataPending = true
		p.cond.Signal()
		p.cond.L.Unlock()

		count += 1
		if count%k_chunkSize == 0 {
			emit(s1, LoadedMsg{count})
		}
	}

	p.cond.L.Lock()
	p.dataReady = true
	p.cond.Broadcast()
	p.cond.L.Unlock()
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryManager) StopFilter(wait bool) {
	p.cond.L.Lock()
	if p.state == Filtering {
		p.state = Stopping
		p.cond.Broadcast()
	}
	for wait && p.state != Stopped {
		p.cond.Wait()
	}
	p.cond.L.Unlock()
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryManager) FilterEntry(s SigRefresh, q string) []EntryNode {
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

	if entry := loadCalculator(q); entry != nil {
		p.storage.emplace(entry)
		q = entry.name
	}

	return p.storage.transform(*p.filterAsync(s, q))
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryManager) filterSync(s SigRefresh, q string) *[]string {
	var strs []string
	foundFn := func(str string) {
		strs = append(strs, str)
		emit(s, RefreshedMsg{p.storage.transform(strs)})
	}
	p.fzfDelegate.ExecuteSync(q, foundFn, p.readSync(0))
	return &strs
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryManager) filterAsync(s SigRefresh, q string) *[]string {
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
		if len(strs)%k_chunkSize == 0 {
			emit(s, RefreshedMsg{p.storage.transform(strs)})
		}
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
			p.fzfDelegate.ExecuteAsync(q, foundFn, readFn))
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
