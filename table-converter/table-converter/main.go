// main.go

package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"

	converter "github.com/milaboratory/small-binaries/table-converter/internal"
)

const (
	optSeparator       = "separator"
	optInputSeparator  = "input-separator"
	optOutputSeparator = "output-separator"

	optSampleColumnName    = "sample-column-name"
	optSampleColumnSearch  = "sample-column-search"
	optSampleColumnI       = "sample-column-i"
	optMetricColumnsSearch = "metric-columns-search"

	optSampleLabel = "sample-label"
	optMetricLabel = "metric-label"
	optValueLabel  = "value-label"
)

func main() {
	flagset := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flagset.Usage = func() { usage(flagset) }

	conf, err := configure(flagset, os.Args[1:])
	if errors.Is(err, flag.ErrHelp) {
		os.Exit(1)
	}

	if err != nil {
		log.Fatal(err.Error())
	}

	converter := converter.New(conf)
	err = converter.Run()
	if err != nil {
		log.Fatal(err)
	}
}

func configure(flags *flag.FlagSet, args []string) (conf converter.Config, err error) {
	var (
		separator       string
		inputSeparator  string
		outputSeparator string

		sampleColumnName string
		sampleColumnRE   string
		sampleColumnI    int
		metricColumnRE   string

		sampleColumnLabel string
		metricColumnLabel string
		valueColumnLabel  string
	)

	flags.StringVar(&separator, optSeparator, "", "Separator for both input and output files")
	flags.StringVar(&inputSeparator, optInputSeparator, "", "Separator for input file")
	flags.StringVar(&outputSeparator, optOutputSeparator, "", "Separator for output file")

	flags.StringVar(&sampleColumnName, optSampleColumnName, "", "Name of the column that contains sample names in input table")
	flags.StringVar(&sampleColumnRE, optSampleColumnSearch, "", "Regex to use when searching the column that contains sample names in input table")
	flags.IntVar(&sampleColumnI, optSampleColumnI, 0, "Instead of searching by name, just use column number N from the table. Left-most column has index 0")
	flags.StringVar(&metricColumnRE, optMetricColumnsSearch, "", "Regex to select metric columns in input table")

	flags.StringVar(&sampleColumnLabel, optSampleLabel, converter.DefaultSampleColumnLabel, "Label for 'sample' column in output table")
	flags.StringVar(&metricColumnLabel, optMetricLabel, converter.DefaultMetricColumnLabel, "Label for 'metric' column in output table")
	flags.StringVar(&valueColumnLabel, optValueLabel, converter.DefaultValueColumnLabel, "Label for 'value' column in output table")

	err = flags.Parse(args)
	if err != nil {
		return conf, err
	}

	if flags.NArg() != 2 {
		return conf, fmt.Errorf("incorrect number of positional parameters: %d instead of %d", flags.NArg(), 2)
	}

	conf.InputFileName = flags.Arg(0)
	conf.OutputFileName = flags.Arg(1)

	if separator != "" {
		conf.InputFileSeparator = rune(separator[0])
		conf.OutputFileSeparator = rune(separator[0])
	}

	//
	// Input parsing options
	//
	if inputSeparator != "" {
		conf.InputFileSeparator = rune(inputSeparator[0])
	}
	if sampleColumnName != "" {
		conf.SampleColumnName = sampleColumnName
	}
	if sampleColumnRE != "" {
		re, err := regexp.Compile(sampleColumnRE)
		if err != nil {
			return conf, converter.Wrapf(err, "%q option contains incorrect regexp. Use go-compatible regular expression", optSampleColumnSearch)
		}
		conf.SampleColumnSearch = re
	}
	if sampleColumnI != 0 {
		conf.SampleColumnIndex = sampleColumnI
	}
	if metricColumnRE != "" {
		re, err := regexp.Compile(metricColumnRE)
		if err != nil {
			return conf, converter.Wrapf(err, "%q option contains incorrect regexp. Use go-compatible regular expression", optMetricColumnsSearch)
		}
		conf.MetricColmunsSearch = re
	}

	//
	// Output file format options
	//
	if outputSeparator != "" {
		conf.OutputFileSeparator = rune(outputSeparator[0])
	}
	if sampleColumnLabel != "" {
		conf.SampleColumnLabel = sampleColumnLabel
	}
	if metricColumnLabel != "" {
		conf.MetricColumnLabel = metricColumnLabel
	}
	if valueColumnLabel != "" {
		conf.ValueColumnLabel = valueColumnLabel
	}

	conf.LoadDefaults()

	return conf, conf.Validate()
}

func usage(flagset *flag.FlagSet) {
	fmt.Printf(
		"Usage:\n\t%s [options] <input-file> <output-file>\n\t\tuse '-' as input/output names to use stdin/stdout.\n\n", flagset.Name())
	flagset.PrintDefaults()
	fmt.Println()
}
