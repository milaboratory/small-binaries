# How to use

```
Usage:
	table-converter [options] <input-file> <output-file>
		use '-' as input/output names to use stdin/stdout.

  -input-separator string
    	Separator for input file
  -metric-columns-search string
    	Regex to select metric columns in input table
  -metric-label string
    	Label for 'metric' column in output table (default "Metric")
  -output-separator string
    	Separator for output file
  -sample-column-i int
    	Instead of searching by name, just use column number N from the table. Left-most column has index 0
  -sample-column-name string
    	Name of the column that contains sample names in input table
  -sample-column-search string
    	Regex to use when searching the column that contains sample names in input table
  -separator string
    	Separator for both input and output files
  -value-label string
    	Label for 'value' column in output table (default "Value")
```
