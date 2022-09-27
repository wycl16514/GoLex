package nfa

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"unicode"
)

type TOKEN int

const (
	ASCII_CHAR_COUNT = 256
)

/*
我们需要对正则表达式字符串进行逐个字符解析，每次读取一个字符时将其转换成特定的token，
这里将不同字符对应的token定义出来
*/
const (
	EOS          TOKEN = iota //读到一行末尾
	ANY                       // .
	AT_BOL                    //^
	AT_EOL                    //$
	CCL_END                   // ]
	CCL_START                 // [
	CLOSE_CURLY               // }
	CLOSE_PARAN               // )
	CLOSURE                   //*
	DASH                      //-
	END_OF_INPUT              // 文件末尾
	L                         //字符常量
	OPEN_CULY                 //{
	OPEN_PAREN                //(
	OPTIONAL                  // ?
	OR                        // |
	PLUS_CLOSE                // +
)

type LexReader struct {
	Verbose        bool   //打印辅助信息
	ActualLineNo   int    //当前读取行号
	LineNo         int    //如果表达式有多行，该变量表明当前读到第几行
	InputFileName  string //读取的文件名
	Lexeme         int    //当前读取字符对应ASCII的数值
	inquoted       bool   //读取到双引号之一
	OutputFileName string
	lookAhead      uint8          //当前读取字符的数值
	tokenMap       []TOKEN        //将读取的字符对应到其对应的token值
	currentToken   TOKEN          //当前字符对应的token
	scanner        *bufio.Scanner //用于读取输入文件，我们需要一行行读取文件内容
	macroMgr       *MacroManager
	currentInput   string   //当前读到的行
	IFile          *os.File //读入的文件
	OFile          *os.File //写出的文件
	lineStack      []string //用于对正则表达式中的宏定义进行展开
	inComment      bool     //是否读取到了注释内容
}

func NewLexReader(inputFile string, outputFile string) (*LexReader, error) {
	reader := &LexReader{
		Verbose:        true,
		ActualLineNo:   0,
		LineNo:         0,
		InputFileName:  inputFile,
		OutputFileName: outputFile,
		Lexeme:         0,
		inquoted:       false,
		currentInput:   "",
		currentToken:   EOS,
		lineStack:      make([]string, 0),
		macroMgr:       GetMacroManagerInstance(),
		inComment:      false,
	}

	var err error
	reader.IFile, err = os.Open(inputFile)
	reader.OFile, err = os.Create(outputFile)
	if err == nil {
		reader.scanner = bufio.NewScanner(reader.IFile)
	}
	reader.initTokenMap()

	return reader, err
}

func (l *LexReader) initTokenMap() {
	l.tokenMap = make([]TOKEN, ASCII_CHAR_COUNT)
	for i := 0; i < len(l.tokenMap); i++ {
		l.tokenMap[i] = L
	}

	l.tokenMap[uint8('$')] = AT_EOL
	l.tokenMap[uint8('(')] = OPEN_PAREN
	l.tokenMap[uint8(')')] = CLOSE_PARAN
	l.tokenMap[uint8('*')] = CLOSURE
	l.tokenMap[uint8('+')] = PLUS_CLOSE
	l.tokenMap[uint8('-')] = DASH
	l.tokenMap[uint8('.')] = ANY
	l.tokenMap[uint8('?')] = OPTIONAL
	l.tokenMap[uint8('[')] = CCL_START
	l.tokenMap[uint8(']')] = CCL_END
	l.tokenMap[uint8('^')] = AT_BOL
	l.tokenMap[uint8('{')] = OPEN_CULY
	l.tokenMap[uint8('|')] = OR
	l.tokenMap[uint8('}')] = CLOSE_CURLY
}

func (l *LexReader) Head() {
	/*
		读取和解析宏定义部分
	*/
	transparent := false
	for l.scanner.Scan() {
		l.ActualLineNo += 1
		l.currentInput = l.scanner.Text()
		if l.Verbose {
			fmt.Printf("h%d: %s\n", l.ActualLineNo, l.currentInput)
		}

		if l.currentInput[0] == '%' {
			if l.currentInput[1] == '%' {
				//头部读取完毕
				l.OFile.WriteString("\n")
				break
			} else {
				if l.currentInput[1] == '{' {
					//拷贝头部代码
					transparent = true
				} else if l.currentInput[1] == '}' {
					//头部代码拷贝完毕
					transparent = false
				} else {
					err := fmt.Sprintf("illegal directive :%s \n", l.currentInput[1])
					panic(err)
				}
			}
		} else if transparent || l.currentInput[0] == ' ' {
			l.OFile.WriteString(l.currentInput + "\n")
		} else {
			//解析宏定义
			l.macroMgr.NewMacro(l.currentInput)
			l.OFile.WriteString("\n")
		}
	}

	if l.Verbose {
		//将当前解析的宏定义打印出来
		l.printMacs()
	}
}

func (l *LexReader) printMacs() {
	l.macroMgr.PrintMacs()
}

func (l *LexReader) Match(t TOKEN) bool {
	return l.currentToken == t
}

func (l *LexReader) Advance() TOKEN {
	/*
			一次读取一个字符然后判断其所属类别，麻烦在于处理转义符和双引号，
		   如果读到 "\s"那么我们要将其对应到空格
	*/
	sawEsc := false //释放看到转义符
	parseErr := NewParseError()
	macroMgr := GetMacroManagerInstance()

	if l.currentToken == EOS {
		if l.inquoted {
			parseErr.ParseErr(E_NEWLINE)
		}

		l.currentInput = l.GetExpr()
		if len(l.currentInput) == 0 {
			l.currentToken = END_OF_INPUT
			return l.currentToken
		}
	}

	/*
		在解析正则表达式字符串时，我们会遇到宏定义，例如：
		  {D}*{D}
		当我们读取到最左边的{时，我们需要将D替换成[0-9]，此时我们需要先将后面的字符串加入栈，
		也就是将字符串"*{D}"放入lineStack，然后讲D转换成[0-9]，接着解析字符串"[0-9]"，
		解析完后再讲原来放入栈的字符串拿出来继续解析
	*/
	for len(l.currentInput) == 0 {
		if len(l.lineStack) == 0 {
			l.currentToken = EOS
			return l.currentToken
		} else {
			l.currentInput = l.lineStack[len(l.lineStack)-1]
			l.lineStack = l.lineStack[0 : len(l.lineStack)-1]
		}
	}

	if !l.inquoted {
		for l.currentInput[0] == '{' { //宏定义里面可能还会嵌套宏定义
			//此时需要展开宏定义
			l.currentInput = l.currentInput[1:]
			expandedMacro := macroMgr.ExpandMacro(l.currentInput)
			var i int
			for i = 0; i < len(l.currentInput); i++ {
				if l.currentInput[i] == '}' {
					break
				}
			}
			l.lineStack = append(l.lineStack, l.currentInput[i+1:])
			l.currentInput = expandedMacro
		}
	}

	if l.currentInput[0] == '"' {
		l.inquoted = !l.inquoted
		l.currentInput = l.currentInput[1:]
		if len(l.currentInput) == 0 {
			l.currentToken = EOS
			return l.currentToken
		}
	}

	sawEsc = l.currentInput[0] == '\\'
	if !l.inquoted {
		if l.currentInput[0] == ' ' {
			l.currentToken = EOS
			return l.currentToken
		}
		/*
			一行内容分为两部分，用空格隔开，前半部分是正则表达式，后半部分是匹配后应该执行的代码
			这里读到第一个空格表明我们完全读取了前半部分，也就是描述正则表达式的部分
		*/
		l.Lexeme = l.esc()
	} else {
		if sawEsc && l.currentInput[1] == '"' {
			//双引号被转义
			l.currentInput = l.currentInput[2:]
			l.Lexeme = int('"')
		} else {
			l.Lexeme = int(l.currentInput[0])
			l.currentInput = l.currentInput[1:]
		}
	}

	if l.inquoted || sawEsc {
		l.currentToken = L
	} else {
		l.currentToken = l.tokenMap[l.Lexeme]
	}

	return l.currentToken
}

func (l *LexReader) esc() int {
	/*
			该函数将转义符转换成对应ASCII码并返回，如果currentInput对应的第一个字符不是反斜杠，那么它直接返回第一个字符
		    然后currentInput递进一个字符。下列转义符将会被处理
		   \b  backspace
		   \f  formfeed
		   \n  newline
		   \r  carriage return
		   \t  tab
		   \e  ESC字符 对应('\0333')
		   \^C C是任何字母，它表示控制符
	*/
	var rval int
	if l.currentInput[0] != '\\' {
		rval = int(l.currentInput[0])
		l.currentInput = l.currentInput[1:]
	} else {
		l.currentInput = l.currentInput[1:] //越过反斜杠
		currentInputUpcase := strings.ToUpper(l.currentInput)
		switch currentInputUpcase[0] {
		case '\x00':
			rval = '\\'
		case 'B':
			rval = '\b'
		case 'F':
			rval = '\f'
		case 'N':
			rval = '\n'
		case 'R':
			rval = '\r'
		case 'S':
			rval = ' '
		case 'T':
			rval = '\t'
		case 'E':
			rval = '\033'
		case '^':
			l.currentInput = l.currentInput[1:]
			upperStr := strings.ToUpper(l.currentInput)
			rval = int(upperStr[0] - '@')
		case 'X':
			rval = 0
			savedCurrentInput := l.currentInput
			transformHex := false
			l.currentInput = l.currentInput[1:]
			if l.isHexDigit(l.currentInput[0]) {
				transformHex = true
				rval = int(l.hex2bin(l.currentInput[0]))
				l.currentInput = l.currentInput[1:]
			}
			if l.isHexDigit(l.currentInput[0]) {
				transformHex = true
				rval <<= 4
				rval |= int(l.hex2bin(l.currentInput[0]))
				l.currentInput = l.currentInput[1:]
			}
			if l.isHexDigit(l.currentInput[0]) {
				transformHex = true
				rval <<= 4
				rval |= int(l.hex2bin(l.currentInput[0]))
				l.currentInput = l.currentInput[1:]
			}
			if !transformHex {
				//如果接在X后面的不是合法16进制字符，那么我们仅仅忽略掉X即可
				l.currentInput = savedCurrentInput
			}
		default:
			if !l.isOctDigit(l.currentInput[0]) {
				rval = int(l.currentInput[0])
				l.currentInput = l.currentInput[1:]
			} else {
				l.currentInput = l.currentInput[1:]
				rval = int(l.oct2bin(l.currentInput[0]))
				savedCurrentInput := l.currentInput
				isTransformOct := false
				l.currentInput = l.currentInput[1:]
				if l.isOctDigit(l.currentInput[0]) {
					isTransformOct = true
					rval <<= 3
					rval |= int(l.oct2bin(l.currentInput[0]))
					l.currentInput = l.currentInput[1:]
				}
				if l.isOctDigit(l.currentInput[0]) {
					isTransformOct = true
					rval <<= 3
					rval |= int(l.oct2bin(l.currentInput[0]))
					l.currentInput = l.currentInput[1:]
				}
				if !isTransformOct {
					l.currentInput = savedCurrentInput
				}
			}
		}
	}

	return rval
}

func (l *LexReader) isHexDigit(x uint8) bool {
	return unicode.IsDigit(rune(x)) || ('a' <= x && x <= 'f') || ('A' <= x && x <= 'F')
}

func (l *LexReader) isOctDigit(x uint8) bool {
	return '0' <= x && x <= '7'
}

func (l *LexReader) hex2bin(x uint8) uint8 {
	/*
		将16进制字符转换为对应数值, x 必须必须是如下字符0123456789abcdefABCDEF
	*/
	var val uint8
	if unicode.IsDigit(rune(x)) {
		val = x - '0'
	} else {
		val = uint8(unicode.ToUpper(rune(x)-'A') & 0xf)
	}

	return val
}

func (l *LexReader) oct2bin(x uint8) uint8 {
	/*
		将十六进制的数字或字母转换为八进制数字，输入的x必须在范围'0'-'7'
	*/
	return (x - '0') & 0x7
}

func (l *LexReader) GetExpr() string {
	/*
		一次从文本中读入一行字符串
	*/
	if l.Verbose {
		fmt.Printf("b:%d\n", l.ActualLineNo)
	}

	readLine := ""
	haveLine := l.scanner.Scan()
	for haveLine {
		currentLine := l.scanner.Text()
		haveLine = l.scanner.Scan()
		if len(strings.TrimSpace(currentLine)) == 0 {
			//忽略掉全是空格的一行
			continue
		}
		if currentLine[0] == uint8(' ') {
			/*
					一个正则表达式可能会分成几行出现，例如 ({D)+ | {D)*\.{D)+ | {D)+\.{D)*) (e{D}+)? 可能分成三行：
				    ({D)+ | {D)*\.{D)+
				       |
				       {D)+\.{D)*) (e{D}+)?

				   第二行和第三行都以空格开始，这种情况我们要将三行内容全部读取，然后合成一行
			*/
			readLine += strings.TrimSpace(currentLine)
		} else {
			readLine = currentLine
			break
		}

	}

	if l.Verbose {
		if !haveLine {
			fmt.Println("----EOF------")
		} else {
			fmt.Println(readLine)
		}
	}

	return readLine
}
