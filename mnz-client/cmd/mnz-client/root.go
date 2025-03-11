// main.go

package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/milaboratory/small-binaries/mnz-client/internal/mnz"
)

const (
	// fakeProductKey is needed for tests of our workflow.
	fakeProductKey = "MIFAKEMIFAKEMIFAKE"

	// fakeResponseToken is a response for fakeProductKey, it contains a number of remaining runs.
	fakeRunSpecResponse = `{"jwtToken": "eyJhbGciOiJFUzI1NiIsImtpZCI6Im1pMiJ9.eyJtbnoiOnsiZGV0YWlscyI6eyJyZW1haW5pbmciOiA5OTk5OTJ9LCJ0eXBlIjoiYmFzZSJ9LCJwcm9kdWN0S2V5IjoiTUlGQUtFTUlGQUtFTUlGQUtFIiwicnVuU3BlYyI6eyJrZXkiOiJ2YWx1ZSJ9fQ==.K7pU8XE476enl-wI-rnHXnvCGLGfM0mdDS0HPdIXhnE5tuc1nKcSZMMTZSZ6USSc1_syHhDkrjsm7UvZTcQwqg"}`

	fakeDryRunResponse = `{
  "httpError": "",
  "response": {
    "result": {
      "productKey": "MIFAKEMIFAKEMIFAKE",
      "canRun": true,
      "mnz": {
        "type": "base",
        "details": {
          "spentRuns": 192,
          "runsToSpend": 1,
          "willRemainAfterRun": 7,
          "subscription": {
            "availableRuns": 200,
            "startsAt": "2025-02-25T11:50:59.000Z",
            "expiresAt": "2025-03-27T11:50:59.000Z"
          }
        }
      }
    }
  }
}`
)

func main() {
	// define flags
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		println("MI_LICENSE=E-ABC mnz-client -productKey test_product [more flags..] <argName>:<type=file>:<filepath>:<specs:size,lines,sha256>")
		println("Only type 'file' now supported.")
		println("Program may send multiple specs. Connect them with comma ','")
		flag.PrintDefaults()
	}
	url := flag.String(
		"url",
		"https://licensing-api.milaboratories.com/mnz/run-spec",
		"Sets URL for sending blocks run statistics",
	)
	dryRun := flag.Bool(
		"dry-run",
		false,
		"Is this run is a dry-run",
	)
	dryRunUrl := flag.String(
		"dry-run-url",
		"https://licensing-api.milaboratories.com/mnz/dry-run",
		"Sets dry-run URL for sending blocks run statistics",
	)
	productKey := flag.String(
		"productKey",
		"",
		"Set your product key, for example: AAAAAXXXXXXXAAAAAXXXXXXXXXX",
	)
	retryWaitMin := flag.Int(
		"retryWaitMin",
		100,
		"Minimum interval in ms between retries",
	)
	retryWaitMax := flag.Int(
		"retryWaitMax",
		1000,
		"Maximum interval in ms between retries",
	)
	retryMax := flag.Int(
		"retryMax",
		3,
		"Maximum number of retries",
	)

	// parse flags
	flag.Parse()

	license, licenseFound := os.LookupEnv("MI_LICENSE")

	// validate flag values
	if productKey == nil || *productKey == "" {
		log.Fatal("Missing mandatory argument: productKey")
	}
	if license == "" || !licenseFound {
		log.Fatal("Missing mandatory env variable, set your private license string: MI_LICENSE=E-ABC..")
	}

	if *productKey == fakeProductKey && *dryRun {
		fmt.Println(fakeDryRunResponse)
		return
	}
	if *productKey == fakeProductKey {
		fmt.Println(fakeRunSpecResponse)
		return
	}

	// prepare call
	mnzArgs, err := mnz.PrepareArgs(flag.Args())
	if err != nil {
		log.Fatal(err)
	}

	// call
	if *dryRun {
		result, err := mnz.CallDryRun(
			*dryRunUrl,
			&mnz.DryRunRequest{
				License:    license,
				ProductKey: *productKey,
				RunSpecs:   []map[string]mnz.Arg{mnzArgs},
			},
			*retryWaitMin, *retryWaitMax, *retryMax,
		)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(string(result))
		return
	} else {
		result, err := mnz.CallRunSpec(
			*url,
			&mnz.RunSpecRequest{
				License:    license,
				ProductKey: *productKey,
				RunSpec:    mnzArgs,
			},
			*retryWaitMin, *retryWaitMax, *retryMax,
		)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(string(result))
		return
	}
}
