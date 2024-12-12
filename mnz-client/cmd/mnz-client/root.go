// main.go

package main

import (
	"flag"
	"fmt"
	"github.com/milaboratory/small-binaries/mnz-client/internal/mnz"
	"log"
	"os"
)

func main() {
	// define flags
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		println("mnz-client -license E-ABC -productName test_product [more flags..] <argName>:<type=file>:<filepath>:<specs:size,linesNum>")
		println("Only type 'file' now supported.")
		println("Program may send multiple specs. Connect them with comma ','")
		flag.PrintDefaults()
	}
	url := flag.String(
		"url",
		"https://licensing-api.milaboratories.com/mnz/run-spec",
		"Sets URL for sending blocks run statistics",
	)
	license := flag.String(
		"license",
		"",
		"Set your private license string",
	)
	productName := flag.String(
		"productName",
		"",
		"Set your product name",
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

	// validate flag values
	if productName == nil || *productName == "" {
		log.Fatal("Missing mandatory argument: productName")
	}
	if license == nil || *license == "" {
		log.Fatal("Missing mandatory argument: license")
	}

	// prepare call
	mnzArgs, err := mnz.PrepareArgs(flag.Args())
	if err != nil {
		log.Fatal(err)
	}

	// call
	req := mnz.RunSpecRequest{
		License:     *license,
		ProductName: *productName,
		RunSpec:     mnzArgs,
	}
	jwt, err := mnz.CallRunSpec(req, *url, *retryWaitMin, *retryWaitMax, *retryMax)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(jwt)
}
