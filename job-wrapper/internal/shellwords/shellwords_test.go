package shellwords

import (
	"reflect"
	"testing"
)

func TestSplit(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"plain", "mixcr align -s hs in.fq out.vdjca", []string{"mixcr", "align", "-s", "hs", "in.fq", "out.vdjca"}},
		{"backend quoting", `'/pkg/mixcr' 'align' '--threads' '4' 'file with space.fq'`, []string{"/pkg/mixcr", "align", "--threads", "4", "file with space.fq"}},
		{"escaped single quote", `'it'\''s' 'x'`, []string{"it's", "x"}},
		{"double quotes", `"a b" "c\"d" "$HOME"`, []string{"a b", `c"d`, "$HOME"}},
		{"backslash outside quotes", `a\ b c`, []string{"a b", "c"}},
		{"empty word", `'' x`, []string{"", "x"}},
		{"tabs and newlines", "a\tb\nc", []string{"a", "b", "c"}},
		{"empty line", "", nil},
		{"operators stay literal", `printf "x\n"; exit 42`, []string{"printf", "x\\n;", "exit", "42"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Split(tc.in)
			if err != nil {
				t.Fatalf("Split(%q): %v", tc.in, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Split(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSplitUnterminated(t *testing.T) {
	for _, in := range []string{`'abc`, `"abc`, `a 'b`} {
		if _, err := Split(in); err == nil {
			t.Errorf("Split(%q): expected error", in)
		}
	}
}
