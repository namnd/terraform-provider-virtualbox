// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"strings"
	"sync"
)

func (c *Client) vmMutex(id string) *sync.Mutex {
	mu, _ := c.vmLocks.LoadOrStore(id, &sync.Mutex{})
	mutex, ok := mu.(*sync.Mutex)
	if !ok {
		mutex = &sync.Mutex{}
		c.vmLocks.Store(id, mutex)
	}
	return mutex
}

func (c *Client) withVMLock(id string, fn func() error) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fn()
	}

	mu := c.vmMutex(id)
	mu.Lock()
	defer mu.Unlock()

	return fn()
}

func withVMLockValue[T any](c *Client, id string, fn func() (T, error)) (T, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return fn()
	}

	mu := c.vmMutex(id)
	mu.Lock()
	defer mu.Unlock()

	return fn()
}
