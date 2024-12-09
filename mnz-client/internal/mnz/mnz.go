package mnz

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hashicorp/go-retryablehttp"
	"io"
	"strings"
	"time"
)

type ArgType struct {
	Name           string                 `json:"name"`
	AvailableSpecs map[string]interface{} `json:"-"`
	RequiredSpecs  []string               `json:"-"`
}

type Arg struct {
	Name    string         `json:"-"`
	ArgType ArgType        `json:"argType"`
	Specs   map[string]any `json:"specs"`
}

func getArgType(s string) (ArgType, error) {
	switch s {
	case "file":
		return FileArgType, nil
	}
	return ArgType{}, errors.New("unknown arg type")
}

type RunSpecRequest struct {
	License     string         `json:"license"`
	ProductName string         `json:"productName"`
	RunSpec     map[string]Arg `json:"runSpec"`
}
type RunSpecResResult struct {
	JwtToken string `json:"jwtToken"`
}
type RunSpecResError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type RunSpecRes struct {
	Result RunSpecResResult `json:"result"`
	Error  RunSpecResError  `json:"error"`
}

func PrepareArgs(args []string) (map[string]Arg, error) {
	const (
		nameN = iota
		typeN
		filepathN
		metricN
	)

	result := make(map[string]Arg, len(args))

	for _, arg := range args {
		splittedArgs := strings.Split(arg, ":")
		if len(splittedArgs) < filepathN { // specs could be empty
			return nil, errors.New("invalid argument, agrument format '<type>:<name>:<filepath>[:metrics]'")
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
		if len(splittedArgs) >= metricN {
			for _, spec := range strings.Split(splittedArgs[metricN], ",") {
				if _, ok := argType.AvailableSpecs[spec]; !ok {
					return nil, fmt.Errorf("invalid spec '%s'", spec)
				}
				ArgSpecNames = append(ArgSpecNames, spec)
			}
		}
		var runSpecs map[string]any
		//for _, specName := range ArgSpecNames {
		switch argType.Name {
		case "file":
			runSpecs, err = fileSpecs(splittedArgs[filepathN], ArgSpecNames)
			if err != nil {
				return nil, fmt.Errorf("error when parsing mnz specs: %w", err)
			}
		default:
			return nil, fmt.Errorf("unknown arg type '%s'", argType.Name)
		}
		//}

		result[argName] = Arg{argName, argType, runSpecs}
	}

	return result, nil
}

func CallRunSpec(
	runSpecRequest RunSpecRequest,
	url string,
	retryWaitMin int,
	retryWaitMax int,
	retryMax int,
) (string, error) {

	// Serialize the request to JSON
	data, err := json.Marshal(runSpecRequest)
	if err != nil {
		return "", fmt.Errorf("failed to serialize request: %w", err)
	}

	// Create a retryable HTTP client
	client := retryablehttp.NewClient()
	client.RetryWaitMin = time.Duration(retryWaitMin) * time.Millisecond
	client.RetryWaitMax = time.Duration(retryWaitMax) * time.Millisecond
	client.RetryMax = retryMax // Max number of retries
	client.Logger = nil

	// Create the POST request
	req, err := retryablehttp.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "mnz-client")

	// Perform the request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check for non-200 HTTP status
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("received non-200 response: %d", resp.StatusCode)
	}

	// Parse the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	jwt, err := unmarshal(body)
	if err != nil {
		return "", err
	}
	return jwt, nil
}

func unmarshal(body []byte) (string, error) {
	result := RunSpecRes{}
	jsonErr := json.Unmarshal(body, &result)
	if jsonErr != nil {
		return "", jsonErr
	}

	if result.Error.Code != "" {
		return "", fmt.Errorf("get API error: %s %s", result.Error.Code, result.Error.Message)
	}
	return result.Result.JwtToken, nil
}
