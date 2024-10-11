package converter

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
)

const SampleColumnName = "Sample"

type Converter struct {
	config Config
}

func New(conf Config) *Converter {
	conf.LoadDefaults()
	return &Converter{config: conf}
}

func (c *Converter) parser(input io.Reader) *csv.Reader {
	r := csv.NewReader(input)
	r.Comma = c.config.InputFileSeparator
	return r
}

func (c *Converter) formatter(output io.Writer) *csv.Writer {
	w := csv.NewWriter(output)
	w.Comma = c.config.OutputFileSeparator
	return w
}

func (c *Converter) Convert(input io.Reader, output io.Writer) error {
	reader := c.parser(input)
	writer := c.formatter(output)
	defer writer.Flush()

	headers, err := reader.Read()
	if err != nil {
		return Wrap(err, "[input]: failed to read header")
	}

	sampleIndex, metricIndices := c.detectColumns(headers)
	if sampleIndex == -1 {
		return errors.New("sample name column not found in input table header")
	}
	if sampleIndex >= len(headers) {
		return fmt.Errorf(
			"sample name column index is outside input table bounds: input columns count is %d, index is %d", len(headers), sampleIndex,
		)
	}

	err = writer.Write([]string{SampleColumnName, c.config.MetricColumnLabel, c.config.ValueColumnLabel})
	if err != nil {
		return Wrap(err, "[output]: failed to write output table header")
	}

	if len(metricIndices) == 0 {
		return nil
	}

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Wrapf(err, "[input]")
		}

		sampleName := record[sampleIndex]
		var metricI = 0
		for _, metricIndex := range metricIndices {
			metricI++
			err = writer.Write([]string{sampleName, headers[metricIndex], record[metricIndex]})
			if err != nil {
				return Wrapf(err, "[output]: failed to write metric %d of sample %q", metricI, sampleName)
			}
		}
	}

	return nil
}

func (c *Converter) detectColumns(headers []string) (sampleIndex int, metricIndices []int) {
	sampleIndex = -1

	if c.config.SampleColumnName != "" {
		for i, header := range headers {
			if header == c.config.SampleColumnName {
				sampleIndex = i
				break
			}
		}
	} else if c.config.SampleColumnSearch != nil {
		for i, header := range headers {
			if c.config.SampleColumnSearch.MatchString(header) {
				sampleIndex = i
				break
			}
		}
	} else {
		sampleIndex = c.config.SampleColumnIndex
	}

	for i, header := range headers {
		if i == sampleIndex {
			continue
		}

		if c.config.MetricColmunsSearch == nil {
			metricIndices = append(metricIndices, i)
			continue
		}

		if c.config.MetricColmunsSearch.MatchString(header) {
			metricIndices = append(metricIndices, i)
		}
	}

	return sampleIndex, metricIndices
}

func (c *Converter) Run() error {
	var input io.Reader = os.Stdin
	if c.config.InputFileName != "-" {
		inputFile, err := os.Open(c.config.InputFileName)
		if err != nil {
			return Wrapf(err, "[input]: failed to open input file %q", c.config.InputFileName)
		}
		defer inputFile.Close()

		input = inputFile
	}

	var output io.Writer = os.Stdout
	if c.config.OutputFileName != "-" {
		outputFile, err := os.Create(c.config.OutputFileName)
		if err != nil {
			return Wrapf(err, "[output]: failed to open output file %q", c.config.OutputFileName)
		}
		defer outputFile.Close()

		output = outputFile
	}

	return c.Convert(input, output)
}
