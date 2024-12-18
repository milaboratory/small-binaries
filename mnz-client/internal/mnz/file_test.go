package mnz

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_fileSpecs(t *testing.T) {
	type args struct {
		path   string
		mNames []string
	}
	tests := []struct {
		name     string
		args     args
		wantSpec map[string]any
		wantErr  string
	}{
		{
			name: "testfile.zip good",
			args: args{
				path:   "testfile.zip",
				mNames: []string{"size", "sha256", "lines"},
			},
			wantSpec: map[string]any{
				"size":   int64(185),
				"sha256": "971780534dd6e29baa46821d52accf486dad1968ea987a86311a0afce885602a",
				"lines":  int64(4),
			},
			wantErr: "",
		},
		{
			name: "testfile.txt good",
			args: args{
				path:   "testfile.txt",
				mNames: []string{"size", "sha256", "lines"},
			},
			wantSpec: map[string]any{
				"size":   int64(19),
				"sha256": "e190705c7bf0c0ada5f6e36fec833f44a0574678267bd86df7e815f3799ecbb1",
				"lines":  int64(4),
			},
			wantErr: "",
		},
		{
			name: "mixcr.csv big",
			args: args{
				path:   "mixcr.csv",
				mNames: []string{"size", "sha256", "lines"},
			},
			wantSpec: map[string]any{
				"size":   int64(42772),
				"sha256": "0d38a35c3f4a0e5e28c17785d114fd74b85596d14e06dc2d951101e6e0683072",
				"lines":  int64(981),
			},
			wantErr: "",
		},
		{
			name: "multi.zip two files",
			args: args{
				path:   "multi.zip",
				mNames: []string{"size", "sha256", "lines"},
			},
			wantSpec: nil,
			wantErr:  "zip multi.zip contains more than one file",
		},
		{
			name: "unknown Spec",
			args: args{
				path:   "multi.zip",
				mNames: []string{"AAA"},
			},
			wantSpec: nil,
			wantErr:  "spec name 'AAA' is not available",
		},
		{
			name: "unknown file",
			args: args{
				path:   "multi123.zip",
				mNames: []string{"size"},
			},
			wantSpec: nil,
			wantErr:  "no such file or directory",
		},
		{
			name: "zipped dir",
			args: args{
				path:   "dir.zip",
				mNames: []string{"lines"},
			},
			wantSpec: nil,
			wantErr:  "zip dir.zip contains directory",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSpec, err := fileSpecs(tt.args.path, tt.args.mNames)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr, "incorrect error from fileSpecs()")
			} else {
				require.NoError(t, err, "unexpected error from fileSpecs()")
			}

			if !reflect.DeepEqual(gotSpec, tt.wantSpec) {
				t.Errorf("fileSpecs() \n"+
					"got  = %v,\n"+
					"want = %v", gotSpec, tt.wantSpec)
			}
		})
	}
}
