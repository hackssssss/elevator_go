package main

import (
	"errors"
	"sync"
)

type DIR int8

const (
	DirUp   DIR = 1
	DirStop DIR = 0
	DirDown DIR = -1
)

type inTask struct {
	who     string
	pressAt int
	dir     DIR
}

type outTask struct {
	who   string
	press int
}

type stayInfo struct {
	dir int
}

type elevator struct {
	level   int
	top     int
	bottom  int
	dir     DIR
	open    bool
	people  map[string]struct{}
	inTask  chan *inTask
	outTask chan *outTask
	stay    map[int]stayInfo
	mu      *sync.Mutex
	runSig chan struct{}
}

func NewElevator(bottom, top int) (*elevator, error) {
	if top <= bottom {
		return nil, errors.New("top or bottom is invalid")
	}
	e := &elevator{
		top:     top,
		bottom:  bottom,
		level:   bottom,
		dir:     0,
		people:  make(map[string]struct{}),
		mu:      &sync.Mutex{},
		inTask:  make(chan *inTask, 30),
		outTask: make(chan *outTask, top-bottom),
		stay:    make(map[int]stayInfo),
		runSig: make(chan struct{},1),
	}
	go e.handleTask()
	return e, nil
}

func (e *elevator) handleTask() {
	go func() {
		for in := range e.inTask {
			e.SetStayInfo(in.pressAt, in.dir)
		}
	}()
	for out := range e.outTask {
		e.SetStayInfo(out.press, DirStop)
	}
}

func (e *elevator) run() {

}

func (e *elevator) SetStayInfo(level int, dir DIR) {
	e.mu.Lock()
	defer e.mu.Unlock()
	cur := 0
	switch dir {
	case DirUp:
		cur |= 1
	case DirDown:
		cur |= 1 << 1
	case DirStop:
		cur |= 1 << 2
	}
	if val, found := e.stay[level]; !found {
		e.stay[level] = stayInfo{
			dir: cur,
		}
	} else {
		val.dir |= cur
		e.stay[level] = val
	}
	if len(e.stay) == 1 {
		e.runSig <- struct{}{}
	}
}

func (e *elevator) Open() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.open
}

func (e *elevator) SetOpen(open bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.open = open
}

func (e *elevator) Level() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.level
}

func (e *elevator) SetLevel(level int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.level = level
}

func (e *elevator) Dir() DIR {
	return e.dir
}

func (e *elevator) SetDir(dir DIR) {
	e.dir = dir
}

func main() {

}
