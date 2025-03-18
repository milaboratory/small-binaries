package mnz

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_unmarshal(t *testing.T) {
	type args struct {
		body []byte
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr error
	}{
		{
			name: "any err",
			args: args{
				body: []byte(`
				{
					"error" : {
						"code" : "VALIDATION_ERR",
						"message" : "bla"
					}
				}
				`),
			},
			want:    nil,
			wantErr: fmt.Errorf("get API error: VALIDATION_ERR bla"),
		},
		{
			name: "jwt",
			args: args{
				body: []byte(`
				{
					"result" : {"jwtToken":"eyJhbGciOiJSUzI1NiJ9.eyJwcm9kdWN0TmFtZSI6InRZWMiOnsiYXNkZiI6IjEyMzQ1In19.bCZuyt1"}
				}
				`),
			},
			want:    []byte(`{"jwtToken":"eyJhbGciOiJSUzI1NiJ9.eyJwcm9kdWN0TmFtZSI6InRZWMiOnsiYXNkZiI6IjEyMzQ1In19.bCZuyt1"}`),
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := unmarshalRunSpec(tt.args.body)
			if err != nil && tt.wantErr == nil ||
				err == nil && tt.wantErr != nil ||
				err != nil && tt.wantErr.Error() != err.Error() {
				t.Errorf("unmarshal() error = %v, wantErr %v", err.Error(), tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("unmarshal(jwtToken) got = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPrepareRunSpecs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    []map[string]Arg
		wantErr bool
	}{
		{
			name: "should ok when single valid file argument",
			args: []string{"0:input:file:mixcr.csv"},
			want: []map[string]Arg{
				{
					"input": {
						Type: ArgTypeFile,
						Name: "input",
						// The actual Spec won't be compared in detail
					},
				},
			},
			wantErr: false,
		},
		{
			name: "should ok when single valid file argument with multiple specs",
			args: []string{"0:input:file:mixcr.csv:size,sha256"},
			want: []map[string]Arg{
				{
					"input": {
						Type: ArgTypeFile,
						Name: "input",
						Spec: map[string]any{
							"size":   nil,
							"sha256": nil,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "should ok when no arguments are provided",
			args:    []string{},
			want:    nil,
			wantErr: false,
		},
		{
			name: "should ok when multiple arguments in same run",
			args: []string{
				"0:input1:file:mixcr.csv",
				"0:input2:file:testfile.txt",
			},
			want: []map[string]Arg{
				{
					"input1": {
						Type: ArgTypeFile,
						Name: "input1",
					},
					"input2": {
						Type: ArgTypeFile,
						Name: "input2",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "should ok when multiple runs",
			args: []string{
				"0:input:file:mixcr.csv",
				"1:output:file:testfile.txt",
			},
			want: []map[string]Arg{
				{
					"input": {
						Type: ArgTypeFile,
						Name: "input",
					},
				},
				{
					"output": {
						Type: ArgTypeFile,
						Name: "output",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "should ok when non-consecutive run indices",
			args: []string{
				"0:input:file:mixcr.csv",
				"2:output:file:testfile.txt",
			},
			want: []map[string]Arg{
				{
					"input": {
						Type: ArgTypeFile,
						Name: "input",
					},
				},
				nil,
				{
					"output": {
						Type: ArgTypeFile,
						Name: "output",
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "should fail when invalid argument format",
			args:    []string{"invalid-format"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "should fail when non-numeric run index",
			args:    []string{"abc:input:file:mixcr.csv"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "should fail when unknown argument type",
			args:    []string{"0:input:unknown:mixcr.csv"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "should fail when with invalid spec",
			args:    []string{"0:input:file:mixcr.csv:invalid_spec"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "should fail when non-existent file",
			args:    []string{"0:input:file:does_not_exist.txt"},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PrepareRunSpecs(tt.args)

			if (err != nil) != tt.wantErr {
				require.FailNow(t, "PrepareRunSpecs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Verify the structure matches without strictly checking Spec contents
			require.Equal(t, len(got), len(tt.want), "PrepareRunSpecs() returned slice of length %d, want %d", len(got), len(tt.want))

			for i := range got {
				if tt.want[i] == nil {
					require.Nil(t, got[i], "PrepareRunSpecs()[%d] = %v, want nil", i, got[i])
					continue
				}

				require.NotNil(t, got[i], "PrepareRunSpecs()[%d] = nil, want %v", i, tt.want[i])

				require.Equal(t, len(got[i]), len(tt.want[i]), "PrepareRunSpecs()[%d] contains %d args, want %d", i, len(got[i]), len(tt.want[i]))

				for argName, wantArg := range tt.want[i] {
					gotArg, exists := got[i][argName]
					require.True(t, exists, "PrepareRunSpecs()[%d] missing expected arg %q", i, argName)
					require.Equal(t, gotArg.Type, wantArg.Type, "PrepareRunSpecs()[%d][%q].Type = %v, want %v", i, argName, gotArg.Type, wantArg.Type)
					require.Equal(t, gotArg.Name, argName, "PrepareRunSpecs()[%d][%q].Name = %v, want %v", i, argName, gotArg.Name, argName)

					// For specs, just check if they exist but don't validate specific contents
					require.NotEmpty(t, gotArg.Spec, "PrepareRunSpecs()[%d][%q].Spec is empty", i, argName)

					// For files with specific metrics requested, check if they're in the result
					if tt.name == "should ok when single valid file argument with multiple specs" {
						require.Contains(t, gotArg.Spec, "size", "PrepareRunSpecs()[%d][%q].Spec missing 'size'", i, argName)
						require.Contains(t, gotArg.Spec, "sha256", "PrepareRunSpecs()[%d][%q].Spec missing 'sha256'", i, argName)
					}
				}
			}
		})
	}
}
