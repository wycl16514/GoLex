package nfa

import (
	"fmt"
	"strings"
)

type Debugger struct {
	level int
}

var DEBUG *Debugger

func newDebugger() *Debugger {
	return &Debugger{
		level: 0,
	}
}

func DebuggerInstance() *Debugger {
	if DEBUG == nil {
		DEBUG = newDebugger()
	}

	return DEBUG
}

func (d *Debugger) Enter(name string) {
	s := strings.Repeat("*", d.level*4) + "entering: " + name
	fmt.Println(s)
	d.level += 1
}

func (d *Debugger) Leave(name string) {
	d.level -= 1
	s := strings.Repeat("*", d.level*4) + "leaving: " + name
	fmt.Println(s)
}
