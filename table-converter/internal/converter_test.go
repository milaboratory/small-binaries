package converter

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

func TestConvert(t *testing.T) {
	type testCase struct {
		input          string
		expectedOutput string
		expectedError  string

		config Config
	}

	tests := map[string]testCase{
		"normal matrix": {
			config: Config{
				InputFileSeparator: ' ',
			},
			input: noindent(`
				Sample M1 M2 M3
				s1 v1 v2 v3
				s2 v4 v5 v6
				s3 v7 v8 v9`),
			expectedOutput: noindent(`
				Sample Metric Value
				s1 M1 v1
				s1 M2 v2
				s1 M3 v3
				s2 M1 v4
				s2 M2 v5
				s2 M3 v6
				s3 M1 v7
				s3 M2 v8
				s3 M3 v9
				`),
		},

		"normal matrix, custom out sep": {
			config: Config{
				InputFileSeparator:  ' ',
				OutputFileSeparator: ',',
				MetricColumnLabel:   "MM",
				ValueColumnLabel:    "VV",
			},

			input: noindent(`
				Sample M1 M2
				s1 v1 v2
				s2 v4 v5
				s3 v7 v8`),
			expectedOutput: noindent(`
				Sample,MM,VV
				s1,M1,v1
				s1,M2,v2
				s2,M1,v4
				s2,M2,v5
				s3,M1,v7
				s3,M2,v8
				`),
		},

		"left-most column as default sample name": {
			config: Config{
				InputFileSeparator: ' ',
				MetricColumnLabel:  "M",
				ValueColumnLabel:   "V",
			},

			input: noindent(`
				Sm M1 M2
				s1 v1 v2`),
			expectedOutput: noindent(`
				Sample M V
				s1 M1 v1
				s1 M2 v2
				`),
		},

		"metric selection": {
			config: Config{
				InputFileSeparator:  ' ',
				OutputFileSeparator: ',',
				MetricColumnLabel:   "M",
				ValueColumnLabel:    "V",
				MetricColmunsSearch: regexp.MustCompile("^M1$"),
			},

			input: noindent(`
				Sample M1 M2
				s1 v1 v2
				s2 v4 v5
				s3 v7 v8`),
			expectedOutput: noindent(`
				Sample,M,V
				s1,M1,v1
				s2,M1,v4
				s3,M1,v7
				`),
		},

		"holey matrix": {
			config: Config{
				InputFileSeparator:  ';',
				OutputFileSeparator: '_',
				SampleColumnName:    "S",

				MetricColumnLabel: "M",
				ValueColumnLabel:  "V",
			},

			input: noindent(`
				M1;S;M2
				v1;s1;
				  ;s2;v2
				v3;s3;v4
				`),
			expectedOutput: noindent(`
				Sample_M_V
				s1_M1_v1
				s1_M2_
				s2_M1_
				s2_M2_v2
				s3_M1_v3
				s3_M2_v4
				`),
		},

		"index selection": {
			config: Config{
				InputFileSeparator:  ';',
				OutputFileSeparator: '_',
				SampleColumnIndex:   1,

				MetricColumnLabel: "M",
				ValueColumnLabel:  "V",
			},

			input: noindent(`
				M1;S;M2
				v1;s1;
				v3;s3;v4
				`),
			expectedOutput: noindent(`
				Sample_M_V
				s1_M1_v1
				s1_M2_
				s3_M1_v3
				s3_M2_v4
				`),
		},

		"empty metrics list": {
			config: Config{
				InputFileSeparator: ' ',
			},

			input: noindent(`
				Sample
				s1
				s2
				s3`),
			expectedOutput: noindent(`
				Sample Metric Value
				`),
		},

		"empty samples": {
			config: Config{
				InputFileSeparator: ' ',
			},

			input: noindent(`
				Sample M2 M1`),
			expectedOutput: noindent(`
				Sample Metric Value
				`),
		},

		"empty values": {
			config: Config{
				InputFileSeparator: ',',
			},

			input: noindent(`
				Sample,M1,M2
				s1,,
				s2,,
				s3,,`),
			expectedOutput: noindent(`
				Sample,Metric,Value
				s1,M1,
				s1,M2,
				s2,M1,
				s2,M2,
				s3,M1,
				s3,M2,
				`),
		},

		"malformed input": {
			config: Config{
				InputFileSeparator: ',',
			},

			input: noindent(`
				Sample,M1,M2
				s1,`),
			expectedError: "line 2",
		},

		"no sample header": {
			config: Config{
				InputFileSeparator: ' ',
				SampleColumnName:   "S",
			},

			input: noindent(`
				U,M1,M2
				s1,v1,v2`),
			expectedError: "sample name column not found in input table header",
		},

		"empty source": {
			config:        Config{InputFileSeparator: ','},
			input:         noindent(``),
			expectedError: "failed to read header",
		},
	}

	runTest := func(conf testCase) func(t *testing.T) {
		return func(t *testing.T) {
			input := strings.NewReader(conf.input)
			output := bytes.NewBuffer(nil)

			converter := New(conf.config)
			err := converter.Convert(input, output)

			if conf.expectedError != "" {
				if !strings.Contains(err.Error(), conf.expectedError) {
					t.Errorf("unexpected error from .Convert: %q does not contain %q", err.Error(), conf.expectedError)
					return
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error from .Convert: %s", err.Error())
				return
			}

			if output.String() != conf.expectedOutput {
				t.Errorf("unexpected .Convert result: expected:\n%s\n\ngot:\n%s", conf.expectedOutput, output.String())
				return
			}
		}
	}

	for name, conf := range tests {
		t.Run(name, runTest(conf))
	}
}

func noindent(in string) string {
	out := strings.Split(in, "\n")
	if len(out) > 0 && out[0] == "" {
		out = out[1:] // cut empty first line to make test cases look nicer
	}

	for i := range out {
		out[i] = strings.TrimSpace(out[i])
	}
	return strings.Join(out, "\n")
}
