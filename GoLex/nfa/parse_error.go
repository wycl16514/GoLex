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
