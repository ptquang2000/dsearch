package dsearch

import (
	"crypto/sha256"
	"encoding/binary"
	"log"
	"sync"
)

///////////////////////////////////////////////////////////////////////////////

type Entry struct {
	name    string
	execute func()
	next    EntryNode
}

type EntryNode interface {
	Value() string
	Execute()
	Next() EntryNode
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

func (p *Entry) Next() EntryNode {
	return p.next
}

///////////////////////////////////////////////////////////////////////////////

type hasher func(string) uint32

type IEntryHashTable interface {
	emplace(*Entry)
	get(string) []EntryNode
	begin() EntryNode
	end() EntryNode
}

type EntryHashTable struct {
	mutex   sync.Mutex
	hash    hasher
	head    *Entry
	tail    *Entry
	storage map[uint32][]EntryNode
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
		hash:    hash(),
		storage: make(map[uint32][]EntryNode),
	}
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryHashTable) emplace(e *Entry) {
	if e == nil {
		log.Fatalf(`Cannot emplace nil entry`)
	}
	p.mutex.Lock()
	defer p.mutex.Unlock()

	key := p.hash(e.name)
	if len(p.storage) == 0 {
		p.head = e
		p.tail = e
	} else {
		p.tail.next = e
		p.tail = e
		p.tail.next = nil
	}
	p.storage[key] = append(p.storage[key], e)
}

////////////////////////////////////////////////////////////////////////////////

func (p *EntryHashTable) get(s string) []EntryNode {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	key := p.hash(s)
	table, ok := p.storage[key]
	if !ok {
		log.Fatalf(`Key %d not found in storage`, key)
	}
	return table
}

////////////////////////////////////////////////////////////////////////////////

func (p *EntryHashTable) begin() EntryNode {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if len(p.storage) == 0 {
		log.Fatalf(`Accessing empty hash table`)
	}
	return p.head
}

////////////////////////////////////////////////////////////////////////////////

func (p *EntryHashTable) end() EntryNode {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if len(p.storage) == 0 {
		log.Fatalf(`Accessing empty hash table`)
	}
	return p.tail
}

////////////////////////////////////////////////////////////////////////////////
