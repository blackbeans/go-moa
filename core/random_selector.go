package core

import (
	"math/rand"
	"sync"
)

type RandomStrategy struct {
	nodes  []string
	length int
	sync.RWMutex
}

func NewRandomStrategy(nodes []string) *RandomStrategy {
	return &RandomStrategy{
		nodes:  nodes,
		length: len(nodes)}
}

func (self *RandomStrategy) ReHash(nodes []string) {
	self.Lock()
	defer self.Unlock()
	self.nodes = nodes
	self.length = len(nodes)
}

func (self *RandomStrategy) Select(key string) string {
	self.RLock()
	defer self.RUnlock()
	return self.nodes[rand.Intn(self.length)]
}

func (self *RandomStrategy) Iterator(f func(idx int, node string)) {
	self.RLock()
	defer self.RUnlock()
	for i, n := range self.nodes {
		f(i, n)
	}
}
