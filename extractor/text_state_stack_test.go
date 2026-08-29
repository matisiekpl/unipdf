package extractor

import (
	"testing"

	"github.com/matisiekpl/unipdf/v3/model"
)

// A "Tf" inside a q...Q block sets the font on the top of the stack, so the block has to be
// discarded before the state is read back. Restoring the top as it stands leaves that font in
// force after the block, and text drawn afterwards is decoded with it.
func TestRestoreUndoesAFontSetInsideTheBlock(t *testing.T) {
	outer, inner := model.DefaultFont(), model.DefaultFont()
	state := textState{tfont: outer}
	var savedStates stateStack
	savedStates.push(&state)

	savedStates.push(&state)
	savedStates.top().tfont = inner
	state.tfont = inner

	savedStates.restore(&state)

	if state.tfont != outer {
		t.Errorf("font set inside the block outlived it")
	}
	if savedStates.top().tfont != outer {
		t.Errorf("stack top still carries the font set inside the block")
	}
}

func TestRestoreKeepsTheBottomEntryForAnUnbalancedQ(t *testing.T) {
	font := model.DefaultFont()
	state := textState{tfont: font}
	var savedStates stateStack
	savedStates.push(&state)

	savedStates.restore(&state)
	savedStates.restore(&state)

	if savedStates.empty() {
		t.Fatal("an unbalanced Q emptied the stack")
	}
	if state.tfont != font {
		t.Errorf("an unbalanced Q lost the current font")
	}
}
