package core

import (
	"sync"
)

type StrategyType string

const (
	STRATEGY_RANDOM = "random"
	STRATEGY_KETAMA = "ketama"
)

type Strategy interface {
	Select(key string) string
	ReHash(nodes []string)
	Iterator(f func(idx int, node string))
}

type KetamaStrategy struct {
	ketama *Ketama
	Nodes  []string
	sync.RWMutex
}

func NewKetamaStrategy(nodes []string) *KetamaStrategy {
	ketama := NewKetama(nodes, len(nodes)*2)
	return &KetamaStrategy{
		ketama: ketama,
		Nodes:  nodes}
}

func (self *KetamaStrategy) ReHash(nodes []string) {
	self.Lock()
	defer self.Unlock()
	ketama := NewKetama(nodes, len(nodes)*2)
	self.ketama = ketama
	self.Nodes = nodes
}

func (self *KetamaStrategy) Select(key string) string {
	self.RLock()
	defer self.RUnlock()
	n := self.ketama.Node(key)
	return n
}

func (self *KetamaStrategy) Iterator(f func(idx int, node string)) {
	self.RLock()
	defer self.RUnlock()
	for i, n := range self.Nodes {
		f(i, n)
	}
}
