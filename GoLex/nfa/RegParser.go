package nfa

import (
	"fmt"
)

const (
	ASCII_CHAR_NUM = 256
)

type RegParser struct {
	debugger  *Debugger
	parseErr  *ParseError
	lexReader *LexReader
	//用于打印NFA状态机信息
	visitedMap map[*NFA]bool
	stateNum   int
}

func NewRegParser(reader *LexReader) (*RegParser, error) {
	regReader := &RegParser{
		debugger:   newDebugger(),
		parseErr:   NewParseError(),
		lexReader:  reader,
		visitedMap: make(map[*NFA]bool),
		stateNum:   0,
	}

	return regReader, nil
}

func (r *RegParser) Parse() *NFA {
	r.lexReader.Advance()
	return r.machine()
}

func (r *RegParser) machine() *NFA {
	/*
		这里进入到正则表达式的解析,其语法规则如下：
		machine -> rule machine | rule END_OF_INPUT
		rule -> expr EOS action | '^'expr EOS action | expr '$' EOS action
		action -> white_space string | white_space | ε
		expr -> expr '|' cat_expr  | cat_expr
		cat_expr -> cat_expr factor | factor
		factor -> term* | term+ | term? | term
		term -> '['string']' | '[' '^' string ']' | '[' ']' | ’[' '^' ']' | '.' | character | '(' expr ')'
		white_space -> 匹配一个或多个空格或tab
		character -> 匹配任何一个除了空格外的ASCII字符
		string -> 由ASCII字符组合成的字符串
	*/
	var p *NFA
	var start *NFA

	r.debugger.Enter("machine")

	start = NewNFA()
	p = start
	p.next = r.rule()

	for !r.lexReader.Match(END_OF_INPUT) {
		p.next2 = NewNFA()
		p = p.next2
		p.next = r.rule()
	}

	r.debugger.Leave("machine")

	return start
}

func (r *RegParser) rule() *NFA {
	/*
		rule -> expr EOS action
		     ->^ expr EOS action
		     -> expr $ EOS action

		action -> <tabs> <characters> epsilon
	*/
	var start *NFA
	var end *NFA
	anchor := NONE

	r.debugger.Enter("rule")

	if r.lexReader.Match(AT_BOL) {
		/*
			当前读到符号 ^,必须开头匹配，因此首先需要读入一个换行符，这样才能确保接下来的字符起始于新的一行
		*/
		start = NewNFA()
		start.edge = EdgeType('\n')
		anchor |= START
		r.lexReader.Advance()
		start, end = r.expr(start.next, end)
	} else {
		start, end = r.expr(start, end)
	}

	if r.lexReader.Match(AT_EOL) {
		/*
			读到符号$，必须是字符串的末尾匹配，因此匹配后接下来必须是回车换行符号，要不然
			无法确保匹配的字符串在一行的末尾
		*/
		r.lexReader.Advance()
		end.next = NewNFA()
		end.edge = CCL //边对应字符集，其中包含符号/r, /n
		end.bitset["\r"] = true
		end.bitset["\n"] = true

		end = end.next
		anchor |= END
	}

	end.accept = r.lexReader.currentInput
	end.anchor = anchor
	r.lexReader.Advance()

	r.debugger.Leave("rule")
	return start
}

func (r *RegParser) expr(start *NFA, end *NFA) (newStar *NFA, newEnd *NFA) {
	/*
		expr -> expr or expr | cat_expr
		一个正则表达式可以分解成两个表达式的并，或是两个表达式的前后连接
	*/
	e2Start := NewNFA()
	e2End := NewNFA()
	var p *NFA
	r.debugger.Enter("expr")

	start, end = r.catExpr(start, end)

	for r.lexReader.Match(OR) {
		r.lexReader.Advance()
		e2Start, e2End = r.catExpr(e2Start, e2End)
		p = NewNFA()
		p.next2 = e2Start
		p.next = start
		start = p

		p = NewNFA()
		end.next = p
		e2End.next = p
		end = p
	}

	r.debugger.Leave("expr")

	return start, end
}

func (r *RegParser) catExpr(start *NFA, end *NFA) (newStart *NFA, newEnd *NFA) {
	/*
		cat_expr -> cat_expr | factor
	*/
	e2Start := NewNFA()
	e2End := NewNFA()
	r.debugger.Enter("catExpr")

	//判断起始字符是否合法
	if r.firstInCat(r.lexReader.currentToken) {
		start, end = r.factor(start, end)
	}

	for r.firstInCat(r.lexReader.currentToken) {
		e2Start, e2End = r.factor(e2Start, e2End)
		end.next = e2Start
		end = e2End
	}

	r.debugger.Leave("catExpr")
	return start, end
}

func (r *RegParser) firstInCat(tok TOKEN) bool {
	switch tok {
	case CLOSE_PARAN:
		fallthrough
	case AT_EOL:
		fallthrough
	case OR:
		fallthrough
	case EOS:
		//这些符号表明正则表达式停止了前后连接过程
		return false
	case CLOSURE:
		fallthrough
	case PLUS_CLOSE:
		fallthrough
	case OPTIONAL:
		//这些字符必须跟在表达式后边而不是作为起始符号
		r.parseErr.ParseErr(E_CLOSE)
		return false
	case CCL_END:
		r.parseErr.ParseErr(E_BRACKET)
		return false
	case AT_BOL:
		r.parseErr.ParseErr(E_BOL)
		return false
	}

	return true
}

func (r *RegParser) factor(start *NFA, end *NFA) (newStart *NFA, newEnd *NFA) {
	/*
		factor -> term* | term+ | term?
	*/
	r.debugger.Enter("factor")
	var e2Start *NFA
	var e2End *NFA
	start, end = r.term(start, end)
	e2Start = start
	e2End = end
	if r.lexReader.Match(CLOSURE) || r.lexReader.Match(PLUS_CLOSE) || r.lexReader.Match(OPTIONAL) {
		e2Start = NewNFA()
		e2End = NewNFA()
		e2Start.next = start
		end.next = e2End

		if r.lexReader.Match(CLOSURE) || r.lexReader.Match(OPTIONAL) {
			//匹配操作符*,+,创建一条epsilon边直接连接头尾
			e2Start.next2 = e2End
		}

		if r.lexReader.Match(CLOSURE) || r.lexReader.Match(PLUS_CLOSE) {
			//匹配操作符*,?，创建一条epsilon边连接尾部和头部
			end.next2 = start
		}

		r.lexReader.Advance()
	}
	r.debugger.Leave("factor")
	return e2Start, e2End
}

func (r *RegParser) printCCL(set map[string]bool) {
	//输出字符集的内容
	s := fmt.Sprintf("%s", "[")
	for i := 0; i <= 127; i++ {
		selected, ok := set[string(i)]
		if !ok {
			continue
		}

		if !selected {
			continue
		}

		if i < int(' ') {
			//控制字符
			s += fmt.Sprintf("^%s", string(i+int('@')))
		} else {
			s += fmt.Sprintf("%s", string(i))
		}
	}

	s += "]"
	fmt.Println(s)
}

func (r *RegParser) PrintNFA(start *NFA) {
	fmt.Println("----------NFA INFO------------")
	nfaNodeStack := make([]*NFA, 0)
	nfaNodeStack = append(nfaNodeStack, start)
	containsMap := make(map[*NFA]bool)
	containsMap[start] = true

	for len(nfaNodeStack) > 0 {
		node := nfaNodeStack[len(nfaNodeStack)-1]
		nfaNodeStack = nfaNodeStack[0 : len(nfaNodeStack)-1]
		fmt.Printf("\n----------In node with state number: %d-------------------\n", node.state)
		r.printNodeInfo(node)

		if node.next != nil && !containsMap[node.next] {
			nfaNodeStack = append(nfaNodeStack, node.next)
			containsMap[node.next] = true
		}

		if node.next2 != nil && !containsMap[node.next2] {
			nfaNodeStack = append(nfaNodeStack, node.next2)
			containsMap[node.next2] = true
		}
	}
}

func (r *RegParser) printNodeInfo(node *NFA) {
	if node.next == nil {
		fmt.Println("this node is TERMINAL")
		return
	}
	fmt.Println("****Edge Info****")
	r.printEdge(node)
	if node.next != nil {
		fmt.Printf("Next node is :%d\n", node.next.state)
	}

	if node.next2 != nil {
		fmt.Printf("Next ode is :%d\n", node.next2.state)
	}
}

func (r *RegParser) printEdge(node *NFA) {
	switch node.edge {
	case CCL:
		r.printCCL(node.bitset)
	case EPSILON:
		fmt.Println("EPSILON")
	default:
		//匹配单个字符
		fmt.Printf("%s\n", string(node.edge))
	}
}

func (r *RegParser) doDash(set map[string]bool) {
	var first int
	for !r.lexReader.Match(EOS) && !r.lexReader.Match(CCL_END) {
		if !r.lexReader.Match(DASH) {
			first = r.lexReader.Lexeme
			set[string(r.lexReader.Lexeme)] = true
		} else {
			r.lexReader.Advance() //越过 '-'
			for ; first <= r.lexReader.Lexeme; first++ {
				set[string(first)] = true
			}
		}
		r.lexReader.Advance()
	}
}

func (r *RegParser) term(start *NFA, end *NFA) (newStart *NFA, newEnd *NFA) {
	/*
		term -> [...] | [^...] | [] | [^] | . | (expr) | <character>
		[] 匹配空格，回车，换行，但不匹配\r
	*/
	r.debugger.Enter("term")

	if r.lexReader.Match(OPEN_PAREN) {
		//匹配(expr)
		r.lexReader.Advance()
		start, end = r.expr(start, end)
		if r.lexReader.Match(CLOSE_PARAN) {
			r.lexReader.Advance()
		} else {
			//没有右括号
			r.parseErr.ParseErr(E_PAREN)
		}
	} else {
		start = NewNFA()
		end = NewNFA()
		start.next = end

		if !(r.lexReader.Match(ANY) || r.lexReader.Match(CCL_START)) {
			//匹配单字符
			start.edge = EdgeType(r.lexReader.Lexeme)
			r.lexReader.Advance()
		} else {
			/*
				匹配 "." 本质上是匹配字符集，集合里面包含所有除了\r, \n 之外的ASCII字符
			*/
			start.edge = CCL
			if r.lexReader.Match(ANY) {
				for i := 0; i < ASCII_CHAR_NUM; i++ {
					if i != int('\r') && i != int('\n') {
						start.bitset[string(i)] = true
					}
				}
			} else {
				/*
					匹配由中括号形成的字符集
				*/
				r.lexReader.Advance() //越过'['
				negativeClass := false
				if r.lexReader.Match(AT_BOL) {
					/*
						[^...] 匹配字符集取反

					*/
					start.bitset[string('\n')] = false
					start.bitset[string('\r')] = false
					negativeClass = true
				}
				if !r.lexReader.Match(CCL_END) {
					/*
						匹配类似[a-z]这样的字符集
					*/
					r.doDash(start.bitset)
				} else {
					/*
						匹配 【】 或 [^]
					*/
					for c := 0; c <= int(' '); c++ {
						start.bitset[string(c)] = true
					}
				}

				if negativeClass {
					for key, _ := range start.bitset {
						start.bitset[key] = false
					}

					for i := 0; i <= 127; i++ {
						_, ok := start.bitset[string(i)]
						if !ok {
							start.bitset[string(i)] = true
						}
					}
				}

				r.lexReader.Advance() //越过 ']'
			}
		}
	}

	r.debugger.Leave("term")
	return start, end
}
