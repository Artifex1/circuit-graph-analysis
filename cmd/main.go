package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/Artifex1/circuit-graph-analysis/internal"
)

func main() {
	// Parse command-line flags
	inputPath := flag.String("input", "", "Input directory or file path")
	parallelism := flag.Int("parallel", runtime.NumCPU(), "Number of parallel workers")
	visualize := flag.Bool("visualize", false, "Whether the Graph should be visualized in HTML")
	flag.Parse()

	if *inputPath == "" {
		fmt.Println("Please provide an input path using the -input flag")
		os.Exit(1)
	}

	// Check if circom is installed
	if err := internal.CheckCircomInstallation(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Get all .circom files
	files, err := internal.GetCircomFiles(*inputPath)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Create an analyzer
	analyzer := internal.NewAnalyzer(*parallelism, *visualize)

	// Process each file
	for _, file := range files {
		if err := analyzer.AnalyzeFile(file); err != nil {
			fmt.Printf("Error analyzing %s: %v\n", file, err)
		}
	}

	// Wait for all analysis to complete
	analyzer.Wait()

	fmt.Println("Analysis complete")
}
