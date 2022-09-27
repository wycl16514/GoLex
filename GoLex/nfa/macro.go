package nfa

import (
	"errors"
	"fmt"
	"strings"
)

type Macro struct {
	//例如  "D  [0-9]" 那么D就是宏定义的名称，[0-9]就是内容
	Name string
	Text string
}

type MacroManager struct {
	macroMap map[string]*Macro
}

var macroManagerInstance *MacroManager

func GetMacroManagerInstance() *MacroManager {
	if macroManagerInstance == nil {
		macroManagerInstance = newMacroManager()
	}

	return macroManagerInstance
}

func newMacroManager() *MacroManager {
	return &MacroManager{
		macroMap: make(map[string]*Macro),
	}
}

func (m *MacroManager) PrintMacs() {
	for _, val := range m.macroMap {
		fmt.Sprintf("mac name: %s, text %s: ", val.Name, val.Text)
	}
}

func (m *MacroManager) NewMacro(line string) (*Macro, error) {
	//输入对应宏定义的一行内容例如 D [0-9]
	line = strings.TrimSpace(line)
	nameAndText := strings.Fields(line)
	if len(nameAndText) != 2 {
		return nil, errors.New("macro string error ")
	}

	/*
		如果宏定义出现重复，那么后面的定义就直接覆盖前面
		例如 :
		D  [0-9]
		D  [a-z]
		那么我们采用最后一个定义也就是D被扩展成[a-z]
	*/
	macro := &Macro{
		Name: nameAndText[0],
		Text: nameAndText[1],
	}

	m.macroMap[macro.Name] = macro
	return macro, nil
}

func (m *MacroManager) ExpandMacro(macroStr string) string {
	/*
			输入: D}, 然后该函数将其转换为[0-9]
		    左括号会被调用函数去除
	*/
	valid := false
	macroName := ""
	for pos, char := range macroStr {
		if char == '}' {
			valid = true
			macroName = macroStr[0:pos]
			break
		}
	}

	if valid != true {
		NewParseError().ParseErr(E_BADREXPR)
	}

	macro, ok := m.macroMap[macroName]
	if !ok {
		NewParseError().ParseErr(E_NOMAC)
	}

	return macro.Text
}
