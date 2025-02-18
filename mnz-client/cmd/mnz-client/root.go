// main.go

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/milaboratory/small-binaries/mnz-client/internal/mnz"
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
		"https://licensing-api.milaboratories.com/mnz/run-spec-dry",
		"Sets dry-run URL for sending blocks run statistics",
	)
	productKey := flag.String(
		"AAAAAXXXXXXXAAAAAXXXXXXXXXX",
		"",
		"Set your product key",
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

	// prepare call
	mnzArgs, err := mnz.PrepareArgs(flag.Args())
	if err != nil {
		log.Fatal(err)
	}

	// call
	req := mnz.RunSpecRequest{
		License:    license,
		ProductKey: *productKey,
		RunSpec:    mnzArgs,
	}
	if *dryRun {
		*url = *dryRunUrl
	}
	result, err := mnz.CallRunSpec(req, *url, *retryWaitMin, *retryWaitMax, *retryMax)
	if err != nil {
		log.Fatal(err)
	}

	bytes, err := json.Marshal(result)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(bytes))
}
