在编译器开发中有两个非常重要的工具名为lex和yacc，他们是编译器的生成器。本质上我们不需要一行行去完成编译器的代码，只需要借助这两个工具，同时制定好词法解析和语法解析的规则后，这两个工具就会自动帮我们把代码生成，我们后续的任务就是使用go语言将这两个工具实现。

为了更好的理解我们要开发的GoLex，我们先熟悉一下lex工具的使用。在Centos上安装lex的命令为:
```
yum install flex
```
接下来我们看看它的使用。lex的作用主要是根据给定正则表达式，然后lex会把既定的正则表达式生成成对应的C语言代码，我们将生成的代码编译后就能得到可以针对输入进行相应识别的程序，我们看看一个具体例子。安装完flex后，我们在本地创建一个名为ch1-02.l的文件，然后输入内容如下：
```
%{
   /*
   这里是一段直接拷贝到c文件的代码
    */
%}
alpha  [a-zA-Z]
%%
[\t]+  /*吸收所有空格*/
is |
am |
are |
where |
was |
be |
being |
been |
does |
did |
will |
would |
should |
can |
could |
has |
have |
had |
go  {printf("%s: is a verb\n", yytext);}
{alpha}+ {printf("%s: is not a verb\n", yytext);}
.|\n   {ECHO; /*normal default anywat*/}
%%
int yywrap(){return 1;}
main() {
    yylex(); //启动识别过程
}
```
文件格式分为三部分，第一部分位于{%  %}之间，这里面用于放置我们自己写的C语言代码，通常情况下是一些变量定义。在%}和%%之间存放正则表达式的宏定义，例如“alpha  [a-zA-Z]”。在%%  %%之间则对应用于识别字符串的正则表达式，这里需要注意的是，在表达式后面还有一段C语言代码，他们会在字符串匹配上之后执行，因此这些代码也称为"Action"。

在第二个%%之后是我们编写的C语言代码，这里面我们定义了yywrap函数，这个函数是lex要求的回调，它的作用我们以后再考虑，在提交的main函数中，我们调用yylex函数启动对输入数据的识别过程，这个函数正是lex通过识别正则表达式，构造状态机，然后将后者转换为可执行代码后对应的函数。

有了上面文件后，我们执行如下命令：
```
lex ch1-02.l
```
完成后就会在本地生成一个名为lex.yy.c的文件，里面的代码非常复杂，同时这些代码完全由lex生成。最后我们用如下命令编译：
```
cc lex.yy.c
```
然后会在本地生成可执行文件a.out，执行a.out后程序运行起来，然后我们就可以输入相应字符串，如果对应字符串满足给定正则表达式，例如输入字符串中包含"should", "would"等单词时，对应的语句printf("%s: is a verb\n", yytext);就可以执行，情况如下图所示：
![请添加图片描述](https://img-blog.csdnimg.cn/a149246fd1bc4d5f9899c99620b1b698.png)
总结来说lex和yacc就是开发编译器的框架，他们负责识别和解析，然后把结果传递给我们提供的函数。接下来我们开发的GoLex就是我们当前介绍lex程序的go语言实现，同时我尝试把其中的C语言转换成python语言。

首先我们创建GoLex工程，然后在其中创建nfa文件夹，然后执行如下命令初始化：
```
go mod init nfa
go mod tidy
```
首先我们看看对应的.l文件，创建input.l，输入内容如下：
```
%{
    FCON = 1
    ICON = 2
%}
D  [0-9]
%%
(e{D}+)?
%%
```
我们将读取上面内容并根据给定的正则表达式创建NFA状态机。我们先在里面创建一些支持类功能的代码，第一个事debugger.go文件，它用来输出函数的调用信息，其内容如下：
```go
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
```
debugger的功能就是显示出函数嵌套调用的次序，前面我们实现的编译器语法解析部分，函数会层级调用，因此有效显示出调用信息会帮助我们更好的查找实现逻辑中的Bug，它展示的信息在我们上一节展示过，当函数嵌套时，被调用函数的输出相对于符函数，它会向右挪到四个字符的位置。

接下来我们实现错误输出功能，创建文件parse_error.go，其内容如下：
```go
package nfa

type ERROR_TYPE int

const (
	E_BADREXPR ERROR_TYPE = iota //表达式字符串有错误
	E_PAREN                      //少了右括号
	E_LENGTH                     //正则表达式数量过多
	E_BRACKET                    //字符集没有以[开始
	E_BOL                        // ^ 必须出现在表达式字符串的起始位置
	E_CLOSE                      //*, +, ? 等操作符前面没有表达式
	E_STRINGS                    //action 代码字符串过长
	E_NEWLINE                    //在双引号包含的字符串中出现回车换行
	E_BADMAC                     //表达式中的宏定义少了右括号}
	E_NOMAC                      //宏定义不存在
	E_MACDEPTH                   //宏定义嵌套太深
)

type ParseError struct {
	err_msgs []string
}

func NewParseError() *ParseError {
	return &ParseError{
		err_msgs: []string{
			"MalFormed regular expression",
			"Missing close parenthesis",
			"Too many regular expressions or expression too long",
			"Missing [ in character class",
			"^ must be at start of expression",
			"Newline in quoted string, use \\n instead",
			"Missing } in macro expansion",
			"Macro doesn't exist",
			"Macro expansions nested too deeply",
		},
	}
}

func (p *ParseError) ParseErr(errType ERROR_TYPE) {
	panic(p.err_msgs[int(errType)])
}

```
上面代码的目的是，在解析过程中发现错误时，我们打印对应的错误信息。接下来我们需要做的是识别输入文件中的宏定义，因此创建文件macro.go，里面实现代码如下：
```go
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

```
上面代码实现的目的是识别宏定义，将其存储在一个map中，后面在解析正则表达式时，一旦遇到宏定义，例如我们定义了宏定义：
```
D  [0-9]
```
然后在后续表达式中遇到宏定义时，例如：
```
(e{D}+)?
```
当读取到{D}时，我们会将其替换成[0-9]，这个替换功能正是由ExpandMacro函数来实现。下面我们先定义状态机节点对应的数据结构，创建nfa.go，实现代码如下：
```go
package nfa

type EdgeType int

const (
	EPSILON EdgeType = iota //epsilon 边
	CCL                     //边对应输入是字符集
)

type Anchor int

const (
	NONE  Anchor = iota
	START        //表达式开头包含符号^
	END          //表达式末尾包含$
	BOTH         //开头包含^同时末尾包含$
)

var NODE_STATE int = 0

type NFA struct {
	edge   EdgeType
	bitset map[string]bool //边对应的输入是字符集例如[A-Z]
	state  int
	next   *NFA //一个nfa节点最多有两条边
	next2  *NFA
	accept string //当进入接收状态后要执行的代码
	anchor Anchor //表达式是否在开头包含^或是在结尾包含$
}

func NewNFA() *NFA {
	node := &NFA{
		edge:   EPSILON,
		bitset: make(map[string]bool),
		next:   nil,
		next2:  nil,
		accept: "",
		state:  NODE_STATE,
		anchor: NONE,
	}

	NODE_STATE += 1
	return node
}
```
其中需要关注的是biteset，它用来实现字符串，如果一条边对应输入为[a-zA-Z]，那么我们就把字符当做键，true当做值插入到这个map，我们还可以执行取反操作，例如[\^a-zA-Z]表示匹配不是字母的字符，这样我们把值设置成false即可。

接下来我们看两个最关键，也是实现难度最大的组件，第一个类似于我们前面实现的lexer，它负责从输入文件中读取数据，然后将读到的字符转换成对应的token，与前面章节不同的是，我们这次一次只读取一个字符进行分析，创建名为LexReader.go文件，由于它的内容比较多，我们一点点展开，首先看其定义部分：
```go
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
    //将对应字符对应到相应的token值
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
```
接下来我们要实现的函数，就是将输入函数中%{  %}对应部分直接拷贝到输出文件，我们使用Head函数来实现，其代码如下：
```go
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
```
在Head函数中，它首先一行行读取输入文件的内容，然后对输入内容进行识别，如果发现读入的行中出现"%%"，这意味着读取任务结束，也就是输入文件的头部内容已经被读取完毕。如果首先读取到符号%，然后接着读取到{，那么我们进入拷贝模式，也就是把所有内容拷贝到输出文件，直到遇到符号 %}为止。一旦读取到%}之后，代码进入到宏定义的识别过程，所有内容在读取符号%%之前都对应宏定义，此时每读取一行内容都是宏定义，代码讲读取的内容输入到l.macroMgr.NewMacro(l.currentInput)，它会将读取内容分解成宏定义的名称和内容然后存储起来。

接下来我们进入到最为复杂的一个函数，它一次读取一个字符，然后将字符转换成对应token，它之所以复杂和繁琐就是因为要处理一些特殊情况，我们先看代码实现：
```go
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
```
这个函数的实现较为复杂，基本逻辑是每次从输入文件中读入一行，然后针对读入的字符串逐个字符分析，这里有几个点需要注意，第一是读取到宏定义时我们需要进行替换，它对应代码为：
```
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
```
一旦读取到左大括号时，上面代码片段就会被执行，它会将大括号里面的字符串取出并将其当做宏定义的名字，然后将宏定义后面的字符串先压入堆栈，然后取出宏对应的内容进行解析。例如读取表达式“(e{D}+)?”,一旦读取到第一个左大括号时，它会把字符D抽取出来，然后将右括号后面的字符串"+)?"压入堆栈，然后获取D对应的定义，也就是字符串"[0-9]"进行解析，一旦这个字符串解析完成后，再从堆栈中取出被压入的内容继续解析，下面代码片段对应从堆栈获取压入的字符串：
```
for len(l.currentInput) == 0 {
		if len(l.lineStack) == 0 {
			l.currentToken = EOS
			return l.currentToken
		} else {
			l.currentInput = l.lineStack[len(l.lineStack)-1]
			l.lineStack = l.lineStack[0 : len(l.lineStack)-1]
		}
	}
```
如果被压入的字符串已经没有内容，那么我们就继续从堆栈上取出内容进行解析。另外还需要考虑的是宏定义里面可能还会包含宏定义，例如：
```
D   [0-9]
DD    {D}
```
上面的定义是合法的，一旦程序解读到DD的时候，它会取出对应内容也就是"{D}"，此时它发现左大括号，于是它再次将括号内的字符串取出，然后将其当做宏定义的名称去查找对应内容，这个操作对应如下片段：
```
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
```
输入解析过程有一些特定情况需要考虑，那就是遇到双引号或者转义符，任何出现在双引号中的字符我们都当做普通字符处理，例如"{D}"，这种情况我们把{D}看做是一个字符串而不是宏定义，同时一旦出现转义符我们也要特殊处理，例如\{D\}，由于左括号和右括号前面都有转义符"\"，因此我们把这两个括号当做普通字符处理，这些情况也是上面代码中两个变量inquoted, 和sawEsc的作用。

代码中有几个变量需要关注，首先是currentToken，它对应当前读到字符对应的token，如果当前读入的字符串已经解析完毕，那么它的值变为EOS(end of string)，另一个变量是Lexeme，它对应当前读入字符的ASCII码数值，currentInput对应当前读入的行。

在上面实现中有一个函数调用需要关注，那就是esc，它的作用是将特定字符进行转义，我们看其实现：
```go
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
```
上面代码实现中有两种情况需要关注，那就是对十六进制和八进制数的转义，当我们读取字符串"\X00f"时，上面代码会将其转换为十进制数15，如果没有x那么就会根据8进制转换，例如“\011"对应的十进制数值就是9，注意到上面代码最多对三个数字进行解析，其他字符的转义可以看代码中的注释。

此外我们看currentInput的设置，它通过调用GetExpr()来获取数据，我们看看这个函数的实现：
```
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

```
上面代码实现中，通过scanner一次读取输入文件的一行文本内容，然后将读入的字符串去除前后空格，如果读入的一行全是由空格组成，那么我们就忽略掉这行并继续读取下一行，另外还需要考虑的是，一个正则表达式可能会分割成多行，那么我们需要将这些不同行的内容合并成一行。

以上内容就是针对输入的读取和解析，它对应于我们前面编译器实例中的词法解析流程。当我们获得输入后就需要识别输入是否满足给定规则，这部分对应前面编译器实例中的语法解析过程，由此我们进入解析过程的实现。

正则表达式字符串的解析跟我们前面编译器实现的语法解析流程一样，我们将字符串中的每个字符转换成对应token之后，就需要判断token的组合是否符合语法规则，由此我们首先给出正则表达式对应的语法规则：
```
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
```
我们看看上面的语法规则如何解析给定正则表达式字符串，例如给定字符串"(e{D}+)",它经过LexReader的处理后{D}会被替换成[0-9]，于是字符串会变成"(e[0-9]+)"，他们转换成token后为: LEFT_PARAN L CCL_START L DASH L CCL_END PLUS_CLOSURE RIGHT_PARAN , 表达式中的字符常量都会转换成L。

我们看看如何使用上面的语法规则解析上面的token序列。首先进入规则machine，它的右边开始是规则rule，因此继续进入到rule。rule规则的右边以expr开始，因此继续进入到规则expr。由于表达式中没有符号'|'，因此进入到expr规则右边的规则cat_expr。在cat_expr中我们会继续进入factor,由于字符串中没有包含符号*, + 和?，因此下一步进入规则term，在规则term中，由于我们第一个字符是左括号，因此此时要匹配规则'(' expr ')'，于是这里我们去除掉标签LEFT_PAREN，然后继续进入到规则expr进行后续匹配。大家可以把上面对正则表达式的识别跟前面我们对四则混合运算表达式的识别对比看看，其实本质上是一样的，符号'|'对应运算表达式中的'+'和'-'，两个表达式前后相连对应计算表达式中的'*'和'\'

不知道大家是否感觉到，所谓语言其实就是提出这一系列解析规则，假如你要开发一门新语言，你只要设计出一组类似这样的语法解析规则就可以了，剩下的就是工程上的开发流程而已。我们看看语法解析的具体实现，创建文件RegParser.go，实现代码如下：
```go
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

```
代码实现的逻辑较为复杂，最好通过视频查看我调试过程才好理解。有几点需要注意，首先是firstInCat，它用来检测表达式的起始字符是否合法，有些字符不能出现在表达式的开始，例如')' ']'等，一旦发现他们出现在开头就意味着表达式的字符串有错误。对上面代码逻辑更详细的拆解请在B站搜索coding迪斯尼。

完成以上代码后，我们就完成了Lex程序的第一部分，后续还有很多工作需要处理，我们在后续章节再进行详解。
