package converter

import (
	"errors"
	"regexp"
)

const (
	DefaultSampleColumnLabel = "Sample"
	DefaultMetricColumnLabel = "Metric"
	DefaultValueColumnLabel  = "Value"
)

type Config struct {
	InputFileName      string
	InputFileSeparator rune

	OutputFileName      string
	OutputFileSeparator rune

	SampleColumnName    string
	SampleColumnSearch  *regexp.Regexp
	SampleColumnIndex   int
	MetricColmunsSearch *regexp.Regexp

	SampleColumnLabel string
	MetricColumnLabel string
	ValueColumnLabel  string
}

func (c *Config) LoadDefaults() {
	if c.InputFileSeparator == 0 {
		c.InputFileSeparator, _ = DetectTableSeparator(c.InputFileName)
	}

	if c.OutputFileSeparator == 0 {
		if c.InputFileSeparator != 0 {
			c.OutputFileSeparator = c.InputFileSeparator
		} else {
			c.OutputFileSeparator, _ = DetectTableSeparator(c.OutputFileName)
		}
	}

	if c.SampleColumnLabel == "" {
		c.SampleColumnLabel = DefaultSampleColumnLabel
	}
	if c.MetricColumnLabel == "" {
		c.MetricColumnLabel = DefaultMetricColumnLabel
	}
	if c.ValueColumnLabel == "" {
		c.ValueColumnLabel = DefaultValueColumnLabel
	}
}

func (c *Config) Validate() error {
	var errs []error
	if c.InputFileSeparator == 0 {
		_, err := DetectTableSeparator(c.InputFileName)
		errs = append(errs, Wrap(err, "[input]"))
	}
	if c.OutputFileSeparator == 0 {
		_, err := DetectTableSeparator(c.OutputFileName)
		errs = append(errs, Wrap(err, "[output]"))
	}

	return errors.Join(errs...)
}
