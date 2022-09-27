package nfa

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAddAndGetMacro(t *testing.T) {
	macroMgr := GetMacroManagerInstance()
	macro, _ := macroMgr.NewMacro("D [0-9]")
	require.Equal(t, macro.Name, "D")
	require.Equal(t, macro.Text, "[0-9]")
}

func TestMacroCoverup(t *testing.T) {
	macroMgr := GetMacroManagerInstance()
	_, _ = macroMgr.NewMacro("D [0-9]")
	macro, _ := macroMgr.NewMacro("D [a-z]")
	require.Equal(t, macro.Text, "[a-z]")
}

func TestNoneMacroPanic(t *testing.T) {
	macroMgr := GetMacroManagerInstance()
	_, _ = macroMgr.NewMacro("D [0-9]")
	assert.Panics(t, func() { macroMgr.ExpandMacro("A}") }, "Macro doesn't exist")
}

func TestMacroExpand(t *testing.T) {
	macroMgr := GetMacroManagerInstance()
	_, _ = macroMgr.NewMacro("D [0-9]")
	text := macroMgr.ExpandMacro("D}")
	require.Equal(t, text, "[0-9]")
}

func TestMacroMissingParam(t *testing.T) {
	macroMgr := GetMacroManagerInstance()
	_, _ = macroMgr.NewMacro("D [0-9]")
	assert.Panics(t, func() { macroMgr.ExpandMacro("D") }, "Missing } in macro expansion")
}
