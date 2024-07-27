package dsearch

import (
	"crypto/sha256"
	"encoding/binary"
	"log"
	"sync"
)

///////////////////////////////////////////////////////////////////////////////

type Entry struct {
	name   string
	action func()
}

///////////////////////////////////////////////////////////////////////////////

type EntryNode struct {
	value  func() string
	action func()
	next   *EntryNode
}

///////////////////////////////////////////////////////////////////////////////

type IEntryLinkedList interface {
	len() int
	begin() *EntryNode
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
		value := func() string { return e.name }
		action := func() {
			if e.action != nil {
				e.action()
			}
		}
		node := &EntryNode{
			value:  value,
			action: action,
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

func (p *EntryLinkedList) append(e *Entry) {
	value := func() string { return e.name }
	action := func() {
		if e.action != nil {
			e.action()
		}
	}
	node := &EntryNode{
		value:  value,
		action: action,
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

func (p *EntryLinkedList) len() int {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return int(p.count)
}

///////////////////////////////////////////////////////////////////////////////

type hasher func(string) uint64

type IEntryHashTable interface {
	emplace(*Entry)
	get(string) []*Entry
}

type EntryHashTable struct {
	mutex   sync.Mutex
	hash    hasher
	storage map[uint64][]*Entry
}

///////////////////////////////////////////////////////////////////////////////

func hash() hasher {
	h := sha256.New()
	return func(s string) uint64 {
		h.Reset()
		h.Write([]byte(s))
		bs := h.Sum(nil)
		return binary.LittleEndian.Uint64(bs)
	}
}

///////////////////////////////////////////////////////////////////////////////

func NewEntryHashTable() IEntryHashTable {
	return &EntryHashTable{
		hash:    hash(),
		storage: make(map[uint64][]*Entry),
	}
}

///////////////////////////////////////////////////////////////////////////////

func (p *EntryHashTable) emplace(e *Entry) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	key := p.hash(e.name)
	if table, ok := p.storage[key]; ok {
		table = append(table, e)
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
		log.Fatalf("Key %d not found in storage", key)
	}
	return table
}

////////////////////////////////////////////////////////////////////////////////
