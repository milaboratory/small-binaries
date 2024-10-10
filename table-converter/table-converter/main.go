// main.go

package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	converter "github.com/milaboratory/small-binaries/table-converter/internal"
)

func main() {
	flagset := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flagset.Usage = func() { usage(flagset) }

	conf, err := configure(flagset, os.Args[1:])
	if errors.Is(err, flag.ErrHelp) {
		os.Exit(0)
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
		inputSeparator    string
		outputSeparator   string
		separator         string
		sampleColumnName  string
		metricColumnLabel string
		valueColumnLabel  string
	)

	flags.StringVar(&inputSeparator, "input-separator", "", "Separator for input file")
	flags.StringVar(&outputSeparator, "output-separator", "", "Separator for output file")
	flags.StringVar(&separator, "separator", "", "Separator for both input and output files")

	flags.StringVar(&sampleColumnName, "sample-column", converter.DefaultSamplesColumnName, "Name of the column that contains sample names in input table")
	flags.StringVar(&metricColumnLabel, "metric-label", converter.DefaultMetricColumnLabel, "Label for 'metric' column in output table")
	flags.StringVar(&valueColumnLabel, "value-label", converter.DefaultValueColumnLabel, "Label for 'value' column in output table")

	err = flags.Parse(args)
	if err != nil {
		return conf, err
	}

	if flags.NArg() != 2 {
		return conf, fmt.Errorf("incorrect number of positional parameters: %d instead of %d", flags.NArg(), 2)
	}

	conf.InputFileName = flags.Arg(0)
	conf.OutputFileName = flags.Arg(1)

	// Override separators if flags provided
	if separator != "" {
		conf.InputFileSeparator = rune(separator[0])
		conf.OutputFileSeparator = rune(separator[0])
	}
	if inputSeparator != "" {
		conf.InputFileSeparator = rune(inputSeparator[0])
	}
	if outputSeparator != "" {
		conf.OutputFileSeparator = rune(outputSeparator[0])
	}

	// Override column names if flags provided
	if sampleColumnName != "" {
		conf.SampleColumnName = sampleColumnName
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
