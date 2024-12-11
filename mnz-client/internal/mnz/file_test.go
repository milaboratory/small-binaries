package mnz

import (
	"fmt"
	"reflect"
	"testing"
)

func Test_fileSpecs(t *testing.T) {
	type args struct {
		path   string
		mNames []string
	}
	tests := []struct {
		name      string
		args      args
		wantSpecs map[string]any
		wantErr   error
	}{
		{
			name: "testfile.zip good",
			args: args{
				path:   "testfile.zip",
				mNames: []string{"size", "hash", "linesNum"},
			},
			wantSpecs: map[string]any{
				"size":     int64(185),
				"hash":     "971780534dd6e29baa46821d52accf486dad1968ea987a86311a0afce885602a",
				"linesNum": int64(4),
			},
			wantErr: nil,
		},
		{
			name: "testfile.txt good",
			args: args{
				path:   "testfile.txt",
				mNames: []string{"size", "hash", "linesNum"},
			},
			wantSpecs: map[string]any{
				"size":     int64(19),
				"hash":     "e190705c7bf0c0ada5f6e36fec833f44a0574678267bd86df7e815f3799ecbb1",
				"linesNum": int64(4),
			},
			wantErr: nil,
		},
		{
			name: "mixcr.csv big",
			args: args{
				path:   "mixcr.csv",
				mNames: []string{"size", "hash", "linesNum"},
			},
			wantSpecs: map[string]any{
				"size":     int64(42772),
				"hash":     "0d38a35c3f4a0e5e28c17785d114fd74b85596d14e06dc2d951101e6e0683072",
				"linesNum": int64(981),
			},
			wantErr: nil,
		},
		{
			name: "multi.zip two files",
			args: args{
				path:   "multi.zip",
				mNames: []string{"size", "hash", "linesNum"},
			},
			wantSpecs: nil,
			wantErr:   fmt.Errorf(""),
		},
		{
			name: "unknown Spec",
			args: args{
				path:   "multi.zip",
				mNames: []string{"AAA"},
			},
			wantSpecs: nil,
			wantErr:   fmt.Errorf(""),
		},
		{
			name: "unknown file",
			args: args{
				path:   "multi123.zip",
				mNames: []string{"size"},
			},
			wantSpecs: nil,
			wantErr:   fmt.Errorf(""),
		},
		{
			name: "zipped dir",
			args: args{
				path:   "dir.zip",
				mNames: []string{"linesNum"},
			},
			wantSpecs: nil,
			wantErr:   fmt.Errorf("zip dir.zip contains directory"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSpecs, err := fileSpecs(tt.args.path, tt.args.mNames)
			if err != nil && tt.wantErr == nil ||
				err == nil && tt.wantErr != nil ||
				tt.wantErr.Error() != err.Error() {
				t.Errorf("\ngotError = %v, \nwantErr  = %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotSpecs, tt.wantSpecs) {
				t.Errorf("fileSpecs() \n"+
					"got  = %v,\n"+
					"want = %v", gotSpecs, tt.wantSpecs)
			}
		})
	}
}
