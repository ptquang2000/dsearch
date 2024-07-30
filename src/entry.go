package dsearch

import (
	"crypto/sha256"
	"encoding/binary"
	"log"
	"runtime/debug"
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

type IEntryLinkedList interface {
	len() int
	begin() EntryNode
	end() EntryNode
	at(int) EntryNode
	emplace_back(*Entry)
	push_back(EntryNode)
}

type EntryLinkedList struct {
	mutex sync.Mutex
	array []*Entry
}

///////////////////////////////////////////////////////////////////////////////

func NewEntryLinkedList() IEntryLinkedList {
	return new(EntryLinkedList)
}

///////////////////////////////////////////////////////////////////////////////

type hasher func(string) uint32

type IEntryHashTable interface {
	IEntryLinkedList
	get(string) []EntryNode
}

type EntryHashTable struct {
	EntryLinkedList
	hash  hasher
	table map[uint32][]EntryNode
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
		table: make(map[uint32][]EntryNode),
	}
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryHashTable) push_back(e EntryNode) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if e == nil {
		debug.PrintStack()
		log.Fatalf(`Cannot emplace nil entry`)
	}

	_e := &Entry{
		name:    e.Value(),
		execute: e.Execute,
		next:    nil,
	}
	key := p.hash(_e.name)
	p.table[key] = append(p.table[key], _e)

	if len(p.array) > 0 {
		p.array[len(p.array)-1].next = _e
	}
	p.array = append(p.array, _e)
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryHashTable) emplace_back(e *Entry) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if e == nil {
		debug.PrintStack()
		log.Fatalf(`Cannot emplace nil entry`)
	}

	key := p.hash(e.name)
	p.table[key] = append(p.table[key], e)

	if len(p.array) > 0 {
		p.array[len(p.array)-1].next = e
	}
	p.array = append(p.array, e)
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryLinkedList) push_back(e EntryNode) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if e == nil {
		debug.PrintStack()
		log.Fatalf(`Cannot emplace nil entry`)
	}

	_e := &Entry{
		name:    e.Value(),
		execute: e.Execute,
		next:    nil,
	}
	if len(p.array) > 0 {
		p.array[len(p.array)-1].next = _e
	}
	p.array = append(p.array, _e)
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryLinkedList) emplace_back(e *Entry) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if e == nil {
		debug.PrintStack()
		log.Fatalf(`Cannot emplace nil entry`)
	}

	if len(p.array) > 0 {
		p.array[len(p.array)-1].next = e
	}
	p.array = append(p.array, e)
}

////////////////////////////////////////////////////////////////////////////////

func (p *EntryHashTable) get(s string) []EntryNode {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	key := p.hash(s)
	table, ok := p.table[key]
	if !ok {
		debug.PrintStack()
		log.Fatalf(`Key %d not found in storage`, key)
	}
	return table
}

////////////////////////////////////////////////////////////////////////////////

func (p *EntryLinkedList) begin() EntryNode {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if len(p.array) == 0 {
		debug.PrintStack()
		log.Fatalf(`Index %d out of range %d`, 0, len(p.array))
	}
	return p.array[0]
}

////////////////////////////////////////////////////////////////////////////////

func (p *EntryLinkedList) end() EntryNode {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if len(p.array) == 0 {
		debug.PrintStack()
		log.Fatalf(`Index %d out of range %d`, 0, len(p.array))
	}
	return p.array[len(p.array)-1]
}

////////////////////////////////////////////////////////////////////////////////

func (p *EntryLinkedList) len() int {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return len(p.array)
}

////////////////////////////////////////////////////////////////////////////////

func (p *EntryLinkedList) at(i int) EntryNode {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if i >= len(p.array) {
		debug.PrintStack()
		log.Fatalf(`Index %d out of range %d`, i, len(p.array))
	}
	return p.array[i]
}

////////////////////////////////////////////////////////////////////////////////
