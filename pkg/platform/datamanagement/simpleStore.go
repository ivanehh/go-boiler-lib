package datamanagement

import (
	"errors"
	"fmt"
)

var (
	ErrNoOrderFound = errors.New("requested item not found")
	ErrNoOverwrite  = errors.New("item already in store")
)

type SimpleStore[K comparable, T any] map[K]T

func NewSimpleStore[K comparable, T any]() SimpleStore[K, T] {
	return make(SimpleStore[K, T])
}

func (os SimpleStore[K, T]) Add(k K, i T) error {
	if _, ok := os[k]; ok {
		return fmt.Errorf("%w; key:%v", ErrNoOverwrite, k)
	}
	os[k] = i
	return nil
}

func (os SimpleStore[K, T]) Get(k K) (T, error) {
	var i T
	var ok bool
	if i, ok = os[k]; !ok {
		return i, fmt.Errorf("%w; key:%v", ErrNoOrderFound, k)
	}
	return i, nil
}

// Update replaces the t value at k; errors if no key not found
func (os SimpleStore[K, T]) Update(k K, i T) error {
	if _, ok := os[k]; !ok {
		return fmt.Errorf("%w; key:%v", ErrNoOrderFound, k)
	}
	os[k] = i
	return nil
}

// Delete deletes the entry at k, including the key; returns error if key not found
func (os SimpleStore[K, T]) Delete(k K) error {
	if _, ok := os[k]; !ok {
		return fmt.Errorf("%w; key:%v", ErrNoOrderFound, k)
	}
	delete(os, k)
	return nil
}
