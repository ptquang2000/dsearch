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
}

///////////////////////////////////////////////////////////////////////////////

type EntryNode struct {
	value   func() string
	execute func()
	next    *EntryNode
}

///////////////////////////////////////////////////////////////////////////////

type IEntryLinkedList interface {
	len() int
	begin() *EntryNode
	end() *EntryNode
	prepend(*Entry)
	append(*Entry)
	appendEntries([]*Entry)
}

///////////////////////////////////////////////////////////////////////////////

type EntryLinkedList struct {
	head  *EntryNode
	tail  *EntryNode
	count int32
	mutex sync.Mutex
}

///////////////////////////////////////////////////////////////////////////////

func NewEntryLinkedList() IEntryLinkedList {
	return new(EntryLinkedList)
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryLinkedList) appendEntries(entries []*Entry) {
	p.mutex.Lock()
	for _, e := range entries {
		if e == nil {
			log.Fatalf(`Cannot append nil entry`)
		}
		node := &EntryNode{
			value:   func() string { return e.name },
			execute: func() { e.execute() },
		}
		if p.count == 0 {
			p.head = node
			p.tail = node
		} else {
			p.tail.next = node
			p.tail = p.tail.next
			p.tail.next = nil
		}
	}
	p.count += int32(len(entries))
	p.mutex.Unlock()
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryLinkedList) prepend(e *Entry) {
	if e == nil {
		log.Fatalf(`Cannot prepend nil entry`)
	}
	node := &EntryNode{
		value:   func() string { return e.name },
		execute: func() { e.execute() },
	}
	p.mutex.Lock()
	if p.count == 0 {
		p.head = node
		p.tail = node
	} else {
		node.next = p.head
		p.head = node
	}
	p.count += 1
	p.mutex.Unlock()
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryLinkedList) append(e *Entry) {
	if e == nil {
		log.Fatalf(`Cannot append nil entry`)
	}
	node := &EntryNode{
		value:   func() string { return e.name },
		execute: func() { e.execute() },
	}
	p.mutex.Lock()
	if p.count == 0 {
		p.head = node
		p.tail = node
	} else {
		p.tail.next = node
		p.tail = p.tail.next
		p.tail.next = nil
	}
	p.count += 1
	p.mutex.Unlock()
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryLinkedList) begin() *EntryNode {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.head
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryLinkedList) end() *EntryNode {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.tail
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryLinkedList) len() int {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return int(p.count)
}

///////////////////////////////////////////////////////////////////////////////

type hasher func(string) uint32

type IEntryHashTable interface {
	emplace(*Entry)
	get(string) []*Entry
}

type EntryHashTable struct {
	mutex   sync.Mutex
	hash    hasher
	storage map[uint32][]*Entry
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
		storage: make(map[uint32][]*Entry),
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
	if table, ok := p.storage[key]; ok {
		p.storage[key] = append(table, e)
		return
	}
	p.storage[key] = make([]*Entry, 0, 1)
	p.storage[key] = append(p.storage[key], e)
}

////////////////////////////////////////////////////////////////////////////////

func (p *EntryHashTable) get(s string) []*Entry {
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
