package mnz

import (
	"fmt"
	"testing"
)

func Test_unmarshal(t *testing.T) {
	type args struct {
		body []byte
	}
	tests := []struct {
		name    string
		args    args
		want    string
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
			want:    "",
			wantErr: fmt.Errorf("get API error: VALIDATION_ERR bla"),
		},
		{
			name: "jwt",
			args: args{
				body: []byte(`
				{
					"result" : {
						"jwtToken" : "eyJhbGciOiJSUzI1NiJ9.eyJwcm9kdWN0TmFtZSI6InRZWMiOnsiYXNkZiI6IjEyMzQ1In19.bCZuyt1"
					}
				}
				`),
			},
			want:    "eyJhbGciOiJSUzI1NiJ9.eyJwcm9kdWN0TmFtZSI6InRZWMiOnsiYXNkZiI6IjEyMzQ1In19.bCZuyt1",
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := unmarshal(tt.args.body)
			if err != nil && tt.wantErr == nil ||
				err == nil && tt.wantErr != nil ||
				err != nil && tt.wantErr.Error() != err.Error() {
				t.Errorf("unmarshal() error = %v, wantErr %v", err.Error(), tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("unmarshal() got = %v, want %v", got, tt.want)
			}
		})
	}
}
