package cmdtemplate

import (
	"reflect"
	"testing"
)

func TestExpandKeepsAValueWithSpacesAsOneArgument(t *testing.T) {
	got, err := Expand(`edit --open "{path}" --wait`, "{path}", []string{`C:\My Projects\app`})
	want := []string{"edit", "--open", `C:\My Projects\app`, "--wait"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestValidateRejectsTemplatesThatCannotRunAProgram(t *testing.T) {
	for _, tpl := range []string{"", "   ", "edit", "{path}", "{path}.exe --x"} {
		if Validate(tpl, "{path}") == nil {
			t.Errorf("%q should be rejected", tpl)
		}
	}
}
