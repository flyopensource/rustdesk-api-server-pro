package policy

import "testing"

func TestValidRootCommand(t *testing.T) {
	for _, value := range []string{"auto", "su", "testsu", "/system/xbin/su", "vendor-su"} {
		if !ValidRootCommand(value) {
			t.Fatalf("expected %q to be valid", value)
		}
	}
	for _, value := range []string{"", "disabled", "su -mm", "su\nnext"} {
		if ValidRootCommand(value) {
			t.Fatalf("expected %q to be invalid", value)
		}
	}
}
