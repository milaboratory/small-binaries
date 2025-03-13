package mnz

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

type ArgTypeName string

const (
	ArgTypeFile ArgTypeName = "file"
)

type ArgType struct {
	Name           ArgTypeName
	AvailableSpecs map[string]interface{}
	RequiredSpecs  []string
}

type Arg struct {
	Type ArgTypeName    `json:"type"`
	Name string         `json:"-"`
	Spec map[string]any `json:"spec"`
}

func getArgType(s string) (ArgType, error) {
	switch s {
	case "file":
		return FileArgType, nil
	}
	return ArgType{}, errors.New("unknown arg type")
}

type DryRunRequest struct {
	License    string           `json:"license"`
	ProductKey string           `json:"productKey"`
	RunSpecs   []map[string]Arg `json:"runSpecs"`
}

// DryRunResult is both a response and http error like connection refused, 500 from milm etc.
// We need to pass it to block's UI as is, so that the component could show this error to the client.
type DryRunResult struct {
	Response  json.RawMessage `json:"response"`
	HTTPError string          `json:"httpError"`
}

type RunSpecRequest struct {
	License    string         `json:"license"`
	ProductKey string         `json:"productKey"`
	RunSpec    map[string]Arg `json:"runSpec"`
}
type RunSpecResError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type RunSpecRes struct {
	// Result might be either jwt token with info or just a ready json in case of dry-run.
	Result json.RawMessage `json:"result"`
	Error  RunSpecResError `json:"error"`
}

// PrepareRunSpecs returns a slice of runs,
// each run could have several named args.
func PrepareRunSpecs(args []string) ([]map[string]Arg, error) {
	const (
		runIndexN = iota
		nameN
		typeN
		filepathN
		metricN
	)

	result := make([]struct {
		argName  string
		runIndex int
		arg      Arg
	}, 0, len(args))
	maxRunIndex := 0

	for _, arg := range args {
		splittedArgs := strings.Split(arg, ":")
		if len(splittedArgs) < filepathN { // specs could be empty
			return nil, errors.New("invalid argument, argument format '<runIndex>:<type>:<name>:<filepath>[:metrics]'")
		}

		// runIndex
		runIndex, err := strconv.Atoi(splittedArgs[runIndexN])
		if err != nil {
			return nil, fmt.Errorf("invalid argument, runIndex is not a number: %w", err)
		}
		if runIndex > maxRunIndex {
			maxRunIndex = runIndex
		}

		// type
		argType, err := getArgType(splittedArgs[typeN])
		if err != nil {
			return nil, fmt.Errorf("error when parsing Arg type '%s': %w", splittedArgs[typeN], err)
		}

		// name
		argName := splittedArgs[nameN]

		// specs
		ArgSpecNames := argType.RequiredSpecs
		if len(splittedArgs) > metricN {
			for _, spec := range strings.Split(splittedArgs[metricN], ",") {
				if _, ok := argType.AvailableSpecs[spec]; !ok {
					return nil, fmt.Errorf("invalid spec '%s'", spec)
				}
				ArgSpecNames = append(ArgSpecNames, spec)
			}
		}
		var runSpecs map[string]any
		// for _, specName := range ArgSpecNames {
		switch argType.Name {
		case ArgTypeFile:
			runSpecs, err = fileSpecs(splittedArgs[filepathN], ArgSpecNames)
			if err != nil {
				return nil, fmt.Errorf("error when calculating mnz specs: %w", err)
			}

		default:
			return nil, fmt.Errorf("unknown arg type '%s'", argType.Name)
		}
		//}

		result = append(result, struct {
			argName  string
			runIndex int
			arg      Arg
		}{
			argName:  argName,
			runIndex: runIndex,
			arg:      Arg{Name: argName, Type: argType.Name, Spec: runSpecs},
		})
	}

	runSpecs := make([]map[string]Arg, maxRunIndex+1)
	for _, arg := range result {
		if runSpecs[arg.runIndex] == nil {
			runSpecs[arg.runIndex] = make(map[string]Arg)
		}
		runSpecs[arg.runIndex][arg.argName] = arg.arg
	}

	return runSpecs, nil
}

func CallDryRun(
	url string,
	req *DryRunRequest,
	retryWaitMin, retryWaitMax, retryMax int,
) ([]byte, error) {
	// Serialize the request to JSON
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize request: %w", err)
	}

	headers := map[string]string{
		"Content-Type": "application/json",
		"User-Agent":   "mnz-client",
	}

	body, err := doHTTPPost(url, data, headers, retryWaitMin, retryWaitMax, retryMax)
	httpError := ""
	if err != nil {
		httpError = err.Error()
	}

	return json.Marshal(DryRunResult{
		Response:  body,
		HTTPError: httpError,
	})
}

func CallRunSpec(
	url string,
	req *RunSpecRequest,
	retryWaitMin, retryWaitMax, retryMax int,
) ([]byte, error) {
	// Serialize the request to JSON
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize request: %w", err)
	}

	headers := map[string]string{
		"Content-Type": "application/json",
		"User-Agent":   "mnz-client",
	}

	body, err := doHTTPPost(url, data, headers, retryWaitMin, retryWaitMax, retryMax)
	if err != nil {
		return nil, fmt.Errorf("failed to do http post: %w", err)
	}

	return unmarshalRunSpec(body)
}

func unmarshalRunSpec(body []byte) ([]byte, error) {
	result := RunSpecRes{}
	jsonErr := json.Unmarshal(body, &result)
	if jsonErr != nil {
		return nil, jsonErr
	}

	if result.Error.Code != "" {
		return nil, fmt.Errorf("get API error: %s %s", result.Error.Code, result.Error.Message)
	}
	return []byte(result.Result), nil
}

func doHTTPPost(
	url string,
	data []byte,
	headers map[string]string,
	retryWaitMin int,
	retryWaitMax int,
	retryMax int,
) ([]byte, error) {
	// Create a retryable HTTP client
	client := retryablehttp.NewClient()
	client.RetryWaitMin = time.Duration(retryWaitMin) * time.Millisecond
	client.RetryWaitMax = time.Duration(retryWaitMax) * time.Millisecond
	client.RetryMax = retryMax // Max number of retries
	client.Logger = nil

	// Create the POST request
	req, err := retryablehttp.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Perform the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check for non-200 HTTP status
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("received non-200 response: %d", resp.StatusCode)
	}

	// Parse the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return body, nil
}
