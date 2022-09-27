package main

import "nfa"

func main() {
	/*
		debugger := nfa.DebuggerInstance()
		debugger.Enter("func1")
		debugger.Enter("func2")
		debugger.Leave("func2")
		debugger.Leave("func1")
	*/

	/*
		errParser := nfa.NewParseError()
		errParser.ParseErr(nfa.E_BADREXPR)
	*/
	//测试读取lex文件头部
	lexReader, _ := nfa.NewLexReader("input.l", "output.py")
	lexReader.Head()
	parser, _ := nfa.NewRegParser(lexReader)
	start := parser.Parse()
	parser.PrintNFA(start)
}
