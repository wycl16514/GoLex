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
