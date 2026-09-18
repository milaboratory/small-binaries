package execid

import (
	"strings"
	"testing"
)

func TestDerive(t *testing.T) {
	e, ok := Derive([]string{"/opt/pkg/bin/mixcr", "align", "--secret", "token123"})
	if !ok {
		t.Fatal("expected ok")
	}
	if e.Command != "mixcr" {
		t.Errorf("Command = %q", e.Command)
	}
	if len(e.ArgsHash) != HashLength {
		t.Errorf("ArgsHash length = %d", len(e.ArgsHash))
	}
	if e.ArgCount != 3 {
		t.Errorf("ArgCount = %d", e.ArgCount)
	}
	if strings.Contains(e.ID(), "token123") {
		t.Errorf("argument value leaked into id %q", e.ID())
	}
	if e.ID() != "mixcr:"+e.ArgsHash {
		t.Errorf("ID = %q", e.ID())
	}
}

func TestDeriveIsStableAndSensitiveToArgs(t *testing.T) {
	a, _ := Derive([]string{"tool", "x", "y"})
	b, _ := Derive([]string{"tool", "x", "y"})
	c, _ := Derive([]string{"tool", "xy"})
	d, _ := Derive([]string{"tool", "x", "z"})
	if a.ArgsHash != b.ArgsHash {
		t.Error("same argv must hash the same")
	}
	if a.ArgsHash == c.ArgsHash {
		t.Error("NUL separation must distinguish [x y] from [xy]")
	}
	if a.ArgsHash == d.ArgsHash {
		t.Error("different args must hash differently")
	}
}

func TestDeriveEmpty(t *testing.T) {
	if _, ok := Derive(nil); ok {
		t.Error("empty argv must not be ok")
	}
	if (Exec{}).ID() != "" {
		t.Error("zero Exec must have empty ID")
	}
}
