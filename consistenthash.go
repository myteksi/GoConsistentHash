/*
Copyright 2016 Dolf Schimmel, Freeaqingme
Copyright 2013 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package GoConsistentHash provides an implementation of a ring hash.
package GoConsistentHash

import (
	"fmt"
	"hash/crc32"
	"sort"
	"strconv"
)

type Hash func(data []byte) uint32

type entry struct {
	weight int
	value  EntryValue
}

type EntryValue interface {
	HashRingId() string
}

type StringValue struct {
	value string
}

func (e *StringValue) HashRingId() string {
	return e.value
}

type Map struct {
	hash          Hash
	defaultWeight int
	keys          []int // Sorted
	hashMap       map[int]string
	entries       map[string]*entry
}

func New(defaultWeight int, fn Hash) *Map {
	m := &Map{
		defaultWeight: defaultWeight,
		hash:          fn,
		hashMap:       make(map[int]string),
		entries:       make(map[string]*entry),
	}
	if m.hash == nil {
		m.hash = crc32.ChecksumIEEE
	}
	return m
}

// Returns true if there are no items available.
func (m *Map) IsEmpty() bool {
	return len(m.keys) == 0
}

// Adds some strings to the hash.
func (m *Map) AddString(keys ...string) error {
	for _, key := range keys {
		if err := m.AddStringWithWeight(key, m.defaultWeight); err != nil {
			return err
		}
	}

	return nil
}

// Adds some strings to the hash with a custom weight.
func (m *Map) AddStringWithWeight(key string, weight int) error {
	return m.AddWithWeight(&StringValue{key}, weight)
}

// Adds an item to the hash.
func (m *Map) AddWithWeight(entryValue EntryValue, weight int) error {
	key := entryValue.HashRingId()
	if _, exists := m.entries[key]; exists {
		return fmt.Errorf("A node with name '%s' already exists", key)
	}
	m.entries[key] = &entry{weight, entryValue}

	for i := 0; i < weight; i++ {
		hash := int(m.hash([]byte(strconv.Itoa(i) + key)))
		m.keys = append(m.keys, hash)
		m.hashMap[hash] = key
	}
	sort.Ints(m.keys)
	return nil
}

func (m *Map) Del(key string) error {
	entry, exists := m.entries[key]
	if !exists {
		return fmt.Errorf("No node with name '%s' found", key)
	}

	for i := 0; i < entry.weight; i++ {
		hash := int(m.hash([]byte(strconv.Itoa(i) + key)))
		delete(m.hashMap, hash)

		for k, v := range m.keys {
			if v == hash {
				m.keys = append(m.keys[:k], m.keys[k+1:]...)
			}
		}
	}

	sort.Ints(m.keys)
	delete(m.entries, key)
	return nil
}

// Gets the N closest items in the hash to the provided key,
// if they're permitted by the accept function. This can be used
// to implement placement strategies like storing items in different
// availability zones.
//
// The accept function returns a bool to indicate whether the item
// is acceptable. Its first argument is the items that have already
// been accepted, the second argument is the item that is about to
// be selected (if accepted).
//
// The AcceptAny and AcceptUnique functions are provided as utility
// functions that can be used as accept-callback.
func (m *Map) GetN(key string, n int, accept func([]string, string) bool) []string {
	out := []string{}
	if m.IsEmpty() || n < 1 {
		return out
	}

	if accept == nil {
		accept = AcceptAny
	}

	hash := int(m.hash([]byte(key)))
	hashKey := m.getKeyFromHash(hash)
	out = append(out, m.hashMap[hashKey])

	ringLength := len(m.hashMap)
	for i := 1; len(out) < n && i < ringLength; i++ {
		hashKey = m.getKeyFromHash(hashKey + 1)
		res := m.hashMap[hashKey]
		if accept(out, res) {
			out = append(out, res)
		}
	}

	return out
}

// Gets the closest item in the hash to the provided key.
func (m *Map) Get(key string) string {
	if m.IsEmpty() {
		return ""
	}

	hash := int(m.hash([]byte(key)))
	return m.hashMap[m.getKeyFromHash(hash)]
}

// Gets the key used in the hashmap based on the provided hash.
func (m *Map) getKeyFromHash(hash int) int {
	// Binary search for appropriate replica.
	idx := sort.Search(len(m.keys), func(i int) bool { return m.keys[i] >= hash })

	// Means we have cycled back to the first replica.
	if idx == len(m.keys) {
		idx = 0
	}

	return m.keys[idx]
}

// Accepts any items when used as accept argument in GetN.
func AcceptAny([]string, string) bool { return true }

// Accepts only unique items when used as accept argument in GetN.
func AcceptUnique(stack []string, found string) bool {
	for _, v := range stack {
		if v == found {
			return false
		}
	}
	return true
}
