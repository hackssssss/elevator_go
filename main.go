package main

import (
	"errors"
	"fmt"
	"sync"
	"time"
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
	level     int
	top       int
	bottom    int
	dir       DIR
	open      bool
	people    map[string]struct{}
	inTask    chan *inTask
	outTask   chan *outTask
	stay      map[int]stayInfo
	mu        *sync.Mutex
	runSig    chan DIR
	broadcast map[int][]chan<- bool
}

func NewElevator(bottom, top int) (*elevator, error) {
	if top <= bottom {
		return nil, errors.New("top or bottom is invalid")
	}
	e := &elevator{
		top:       top,
		bottom:    bottom,
		level:     bottom,
		dir:       0,
		people:    make(map[string]struct{}),
		mu:        &sync.Mutex{},
		inTask:    make(chan *inTask, 300),
		outTask:   make(chan *outTask, top-bottom),
		stay:      make(map[int]stayInfo),
		runSig:    make(chan DIR, 1),
		broadcast: make(map[int][]chan<- bool),
	}
	go e.handleTask()
	return e, nil
}

// 处理进出任务
func (e *elevator) handleTask() {
	go func() {
		for in := range e.inTask {
			e.SetStayInfo(in.pressAt, in.dir)
		}
	}()
	go e.run()
	for out := range e.outTask {
		e.SetStayInfo(out.press, DirStop)
	}
}

func (e *elevator) openElevator() {
	e.open = true
	fmt.Printf("电梯已开门, 楼层：%d，方向：%s\n", e.level, e.getDirText(e.dir))
	time.Sleep(time.Second * 2)
	e.open = false
	fmt.Println("电梯已关闭")
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
		if e.level == level {
			return
		}
		cur |= 1 << 2
	}
	start := false
	if len(e.stay) == 0 {
		start = true
	}
	if val, found := e.stay[level]; !found {
		e.stay[level] = stayInfo{
			dir: cur,
		}
	} else {
		val.dir |= cur
		e.stay[level] = val
	}
	if start {
		if level > e.level {
			e.runSig <- DirUp
		} else {
			e.runSig <- DirDown
		}
	}
}

func (e *elevator) Open() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.open
}

func (e *elevator) run() {
	for {
		fmt.Println("电梯等待运行", e.GetLevel(), e.getDirText(e.GetDir()), e.stay)
		dir := <-e.runSig
		e.SetDir(dir)
		fmt.Printf("电梯开始启动……,目前在%d楼, dir: %s\n", e.GetLevel(), e.getDirText(dir))
		for e.hasTask() {
			time.Sleep(time.Second * 2)
			e.goNextLevel()
			e.checkStay()
		}
	}
}

func (e *elevator) checkStay() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if v, found := e.stay[e.level]; found {
		switch e.dir {
		case DirUp: // 电梯向上走
			if (v.dir&1 > 0) || (v.dir&(1<<2) > 0) {
				e.openElevator()
				v.dir &= ^1
				v.dir &= ^(1 << 2)
			}
		case DirDown:
			if v.dir&(1<<1) > 0 || (v.dir&(1<<2) > 0) {
				e.openElevator()
				v.dir &= ^(1 << 1)
				v.dir &= ^(1 << 2)
			}
		case DirStop:
			if v.dir > 0 {
				e.openElevator()
				v.dir = 0
			}
		}
		if v.dir == 0 {
			delete(e.stay, e.level)
			fmt.Println("楼层任务完成", e.level)
		} else {
			e.stay[e.level] = v
		}
	}
	if e.dir == DirStop && len(e.stay) > 0 {
		i := e.bottom
		for ; i <= e.top; i++ {
			if _, found := e.stay[i]; found {
				break
			}
		}
		if i < e.level {
			e.dir = DirDown
		} else {
			e.dir = DirUp
		}
	}
}

func (e *elevator) goNextLevel() (newDir DIR, nowLevel int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	newDir = e.dir
	switch e.dir {
	case DirUp:
		top := e.level
		for i := e.top; i >= e.level; i-- {
			if _, found := e.stay[i]; found {
				top = i
				break
			}
		}
		if e.level == top {
			newDir = DirStop
		}
	case DirDown:
		bottom := e.level
		for i := e.bottom; i <= e.level; i++ {
			if _, found := e.stay[i]; found {
				bottom = i
				break
			}
		}
		if e.level == bottom {
			newDir = DirStop
		}
	}
	e.dir = newDir
	e.level += (int)(e.dir)
	fmt.Printf("电梯已经到达%d楼, 方向%s\n", e.level, e.getDirText(e.dir))
	fmt.Println(e.stay)
	// notify
	fmt.Println(e.broadcast)
	if len(e.broadcast[e.level]) > 0 {
		for _, ch := range e.broadcast[e.level] {
			select {
			case ch <- true:
			case <-time.After(time.Millisecond * 100):
				fmt.Println("超过100ms没有写入channel")
			}
		}
		// 清除通知
		delete(e.broadcast, e.level)
	}

	return newDir, e.level
}

func (e *elevator) hasTask() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.stay) > 0
}

func (e *elevator) SetOpen(open bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.open = open
}

func (e *elevator) GetLevel() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.level
}

func (e *elevator) SetLevel(level int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.level = level
}

func (e *elevator) GetDir() DIR {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.dir
}

func (e *elevator) SetDir(dir DIR) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.dir = dir
}

func (e *elevator) getDirText(dir DIR) string {
	switch dir {
	case DirUp:
		return "向上"
	case DirDown:
		return "向下"
	case DirStop:
		return "停留"
	}
	return "unknown dir"
}

// 在外部按电梯
func (e *elevator) Press(dir DIR, level int, who string) (arrived <-chan bool) {
	fmt.Printf("%s在%d楼按了%s方向的电梯\n", who, level, e.getDirText(dir))
	switch dir {
	case DirUp:
		if level >= e.top {
			return
		}
	case DirDown:
		if level <= e.bottom {
			return
		}
	}
	task := &inTask{
		who:     who,
		pressAt: level,
		dir:     DirUp,
	}
	e.inTask <- task
	ch := make(chan bool, 1)
	e.addListener(ch, level)
	return ch
}

func (e *elevator) addListener(notifyCh chan<- bool, level int) {
	if notifyCh == nil {
		panic("notifyCh is nil")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.broadcast[level] = append(e.broadcast[level], notifyCh)
}

func (e *elevator) InnerPress(target int, who string) {
	fmt.Printf("%s要去%d楼\n", who, target)
	if target > e.top || target < e.bottom {
		return
	}
	task := outTask{
		who:   who,
		press: target,
	}
	e.outTask <- &task
}

func main() {
	e, err := NewElevator(-3, 25)
	if err != nil {
		panic(err)
	}
	wait := e.Press(DirUp, 1, "lyl")
	select {
	case <-wait:
		fmt.Println("lyl已经等到电梯")
	}
	time.Sleep(time.Second)
	e.InnerPress(5, "lyl")

	go func() {
		time.Sleep(time.Second * 10)
		wait2 := e.Press(DirUp, 1, "lyl2")
		select {
		case <-wait2:
			fmt.Println("lyl2已经等到电梯")
		}
		e.InnerPress(6, "lyl2")
		time.Sleep(time.Second)
		wait3 := e.Press(DirDown, 8, "lyl3")
		<-wait3
		fmt.Println("lyl3已经等到电梯")
		e.InnerPress(1, "lyl3")
	}()

	time.Sleep(time.Hour)
}
