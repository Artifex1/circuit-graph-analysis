# Circuit Graph Analysis

The **Circuit Template Analyzer** is a proof of concept to analyze Circom templates.

## Features

- Template Extraction: Automatically identifies and processes circuit templates within the files.
- Graph Analysis: Build constraint graphs from compiled circuits and identify critical issues:
    - Signals with insufficient connections (potential underconstraints). Note that this still includes input signals (FPs).
    - Independent subgraphs in the circuit (potential modularity or underconstraint issues).
- Visualization: Optionally generate HTML-based visualizations of the constraint graph.
- Parallel Processing: Analyze multiple Circom files concurrently using a worker pool.

## Usage

Download the `circuit-analyzer` binary, or compile from source:

```
go build -o circuit-analyzer cmd/main.go
```

Ensure you have `go-echarts` and `gonum` installed:

```
go get -u github.com/go-echarts/go-echarts/v2
go get -u gonum.org/v1/gonum/graph
```

You can run the tool on a specific directory or file using:

```
./circuit-analyzer --input <file_path> [--parallelism=N] [--visualize]
<file_path>: Path to the Circom file or directory containing files you want to analyze.
--parallelism=N: Optional. Defines the number of files to analyze concurrently (default: all CPUs).
--visualize: Optional. Enables visualization of the circuit constraint graphs in HTML format. (default: false).
```

## Example Output

```
Analyzing template MyTemplate from template.circom
Graph Analysis:
There are 100 nodes (signals) in this graph.
Potentially underconstrained signals (one or no connections): [signal_a, signal_b]
Warning: Found 2 independent subgraphs. The circuit might be underconstrained or should be broken into separate templates.
```

Additionally, the HTML files for visualization will be created in the current working directory.
