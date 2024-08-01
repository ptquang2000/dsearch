package dsearch

import (
	"crypto/sha256"
	"encoding/binary"
	"log"
	"runtime/debug"
	"slices"
	"sync"
)

///////////////////////////////////////////////////////////////////////////////

type Entry struct {
	name    string
	execute func()
}

type EntryNode interface {
	Value() string
	Execute()
}

///////////////////////////////////////////////////////////////////////////////

func (p *Entry) Value() string {
	return p.name
}

///////////////////////////////////////////////////////////////////////////////

func (p *Entry) Execute() {
	if p.execute != nil {
		p.execute()
	}
}

///////////////////////////////////////////////////////////////////////////////

type hasher func(string) uint32

type IEntryHashTable interface {
	traverse(start int, callback func(string) bool)
	forEach(start, end int, callback func(string) bool)
	transform(strs []string) []EntryNode
	getRawData() []EntryNode
	emplace(e *Entry)
	len() int
}

type EntryHashTable struct {
	mutex sync.Mutex
	array []EntryNode
	hash  hasher
	table map[uint32][]int
}

///////////////////////////////////////////////////////////////////////////////

func hash() hasher {
	h := sha256.New()
	return func(s string) uint32 {
		h.Reset()
		h.Write([]byte(s))
		bs := h.Sum(nil)
		return binary.LittleEndian.Uint32(bs)
	}
}

///////////////////////////////////////////////////////////////////////////////

func NewEntryHashTable() IEntryHashTable {
	return &EntryHashTable{
		hash:  hash(),
		table: make(map[uint32][]int),
	}
}

func (p *EntryHashTable) len() int {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return len(p.array)
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryHashTable) getRawData() []EntryNode {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.array
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryHashTable) emplace(e *Entry) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if e == nil {
		debug.PrintStack()
		log.Fatalf(`Cannot emplace nil entry`)
	}

	key := p.hash(e.Value())
	if indexes, ok := p.table[key]; !ok {
		p.table[key] = append(p.table[key], len(p.array))
	} else if !slices.ContainsFunc(indexes, func(i int) bool {
		return p.array[i].Value() == e.Value()
	}) {
		p.table[key] = append(p.table[key], len(p.array))
	} else {
		return
	}
	p.array = append(p.array, e)
}

////////////////////////////////////////////////////////////////////////////////

func (p *EntryHashTable) transform(strs []string) []EntryNode {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	var indexes []int
	for _, str := range strs {
		key := p.hash(str)
		if i, ok := p.table[key]; ok {
			indexes = append(indexes, i...)
		} else {
			debug.PrintStack()
			log.Fatalf(`Key %d not found in storage`, key)
		}
	}
	slices.Sort(indexes)
	var nodes []EntryNode
	for _, idx := range indexes {
		nodes = append(nodes, p.array[idx])
	}
	return nodes
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryHashTable) forEach(start, end int, callback func(string) bool) {
	for i := start; i < end; i++ {
		if !callback(p.array[i].Value()) {
			break
		}
	}
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryHashTable) traverse(start int, callback func(string) bool) {
	for i := start; i < p.len(); i++ {
		if !callback(p.array[i].Value()) {
			break
		}
	}
}

///////////////////////////////////////////////////////////////////////////////
