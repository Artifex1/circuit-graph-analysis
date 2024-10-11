package internal

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/simple"
	"gonum.org/v1/gonum/graph/topo"
)

type Analyzer struct {
	workerPool chan struct{}
	wg         sync.WaitGroup
	visualize  bool
}

func NewAnalyzer(parallelism int, visualize bool) *Analyzer {
	return &Analyzer{
		workerPool: make(chan struct{}, parallelism),
		visualize:  visualize,
	}
}

func (a *Analyzer) AnalyzeFile(filePath string) error {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.workerPool <- struct{}{}        // Acquire a worker
		defer func() { <-a.workerPool }() // Release the worker

		if err := a.processFile(filePath); err != nil {
			fmt.Printf("Error processing %s: %v\n", filePath, err)
		}
	}()
	return nil
}

func (a *Analyzer) processFile(filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	templates := extractTemplates(string(content))

	for _, template := range templates {
		if err := a.analyzeTemplate(filePath, template); err != nil {
			fmt.Printf("Error analyzing template %s in %s: %v\n", template.Name, filePath, err)
		}
	}

	return nil
}

func (a *Analyzer) analyzeTemplate(filePath string, template TemplateInfo) error {
	tempFile, err := CreateTempCircomFile(filePath)
	if err != nil {
		return err
	}
	defer os.Remove(tempFile)

	args := GenerateRandomArgs(template.ArgCount)
	if err := AddMainComponent(tempFile, template.Name, args); err != nil {
		return err
	}

	constraintsFile, symFile, err := CompileCircuit(tempFile)
	if err != nil {
		return err
	}
	defer os.Remove(constraintsFile)
	defer os.Remove(symFile)

	fmt.Printf("\nAnalyzing template %s from %s\n", template.Name, filePath)

	constraints, err := LoadFromJson(constraintsFile)
	if err != nil {
		return err
	}
	signals, err := LoadFromSym(symFile)
	if err != nil {
		return err
	}

	graph := buildGraph(constraints, signals)
	if a.visualize {
		visualizeGraph(graph, template.Name)
	}
	analyzeGraph(graph)

	return nil
}

func (a *Analyzer) Wait() {
	a.wg.Wait()
}

type TemplateInfo struct {
	Name     string
	ArgCount int
}

func extractTemplates(content string) []TemplateInfo {
	// Update the regular expression to capture template names and arguments
	re := regexp.MustCompile(`(?m)^\s*template\s+(\w+)\(([^)]*)\)`)
	matches := re.FindAllStringSubmatch(content, -1)

	templates := make([]TemplateInfo, len(matches))
	for i, match := range matches {
		templateName := match[1]
		args := match[2]

		// Count the arguments by splitting on commas, and handling empty arguments
		argCount := 0
		if strings.TrimSpace(args) != "" {
			argCount = len(strings.Split(args, ","))
		}

		templates[i] = TemplateInfo{
			Name:     templateName,
			ArgCount: argCount,
		}
	}

	return templates
}

type NamedNode struct {
	IDVal int64  // Node ID
	Name  string // Node name or title
}

// ID satisfies the gonum Node interface
func (n NamedNode) ID() int64 {
	return n.IDVal
}

func buildGraph(data Constraints, signals []string) *simple.UndirectedGraph {
	graph := simple.NewUndirectedGraph()

	for _, constraint := range data {
		// Collect all unique signals in this constraint
		signalSet := make(map[int64]struct{})
		for _, linearExpression := range constraint {
			for _, signal := range linearExpression {
				signalSet[signal] = struct{}{}
			}
		}

		// Create or get nodes for all signals in this constraint
		nodes := make([]*NamedNode, 0, len(signalSet))
		for signal := range signalSet {
			node, ok := graph.Node(signal).(*NamedNode)
			if !ok {
				// Add node if it doesn't exist
				node = &NamedNode{IDVal: signal, Name: signals[signal]}
				graph.AddNode(node)
			}
			nodes = append(nodes, node)
		}

		// Connect all nodes with each other
		for i := 0; i < len(nodes); i++ {
			for j := i + 1; j < len(nodes); j++ {
				graph.SetEdge(simple.Edge{F: nodes[i], T: nodes[j]})
			}
		}
	}

	return graph
}

func visualizeGraph(dataGraph *simple.UndirectedGraph, templateName string) {
	viewGraph := charts.NewGraph()
	viewGraph.SetGlobalOptions(charts.WithTitleOpts(opts.Title{Title: "Circuit Constraint Graph: " + templateName}))

	nodes := make([]opts.GraphNode, 0)
	links := make([]opts.GraphLink, 0)

	nodesIterator := dataGraph.Nodes()
	for nodesIterator.Next() { // Loop through the nodes
		n := nodesIterator.Node().(*NamedNode) // Get the current node
		nodes = append(nodes, opts.GraphNode{
			Name: fmt.Sprintf(n.Name), // Format the node name
		})
	}

	edgesIterator := dataGraph.Edges()
	for edgesIterator.Next() {
		e := edgesIterator.Edge() // Type assertion to get the edge details
		sourceNode := e.From()
		targetNode := e.To()

		// Convert source and target nodes to *NamedNode to access the Name attribute
		sourceNamedNode, _ := sourceNode.(*NamedNode)
		targetNamedNode, _ := targetNode.(*NamedNode)

		links = append(links, opts.GraphLink{
			Source: fmt.Sprintf(sourceNamedNode.Name),
			Target: fmt.Sprintf(targetNamedNode.Name),
		})
	}

	viewGraph.AddSeries("graph", nodes, links)
	fileName := fmt.Sprintf("%s_circuit_graph.html", templateName)
	f, _ := os.Create(fileName)
	viewGraph.Render(f)
}

func analyzeGraph(g *simple.UndirectedGraph) {
	fmt.Printf("There are %d nodes (signals) in this graph.\n", g.Nodes().Len())

	// Check for signals with one or no connections
	underconstrained := findUnderconstrainedSignals(g)
	if len(underconstrained) > 0 {
		fmt.Println("Potentially underconstrained signals (one or no connections):", underconstrained)
	} else {
		fmt.Println("No potentially underconstrained signals found.")
	}

	// Create a copy of the graph for subgraph analysis
	gc := simple.NewUndirectedGraph()
	graph.Copy(gc, g)

	// Remove node 0 from the copy, which is the "1" signal
	gc.RemoveNode(int64(0))

	// Check for independent subgraphs in the modified copy
	subgraphs := topo.ConnectedComponents(gc)
	if len(subgraphs) > 1 {
		fmt.Printf("Found %d independent subgraphs after removing node 0. The circuit might be underconstrained or should be broken into separate templates.\n", len(subgraphs))
		for i, subgraph := range subgraphs {
			fmt.Printf("Subgraph %d:\n", i+1)
			for _, node := range subgraph {
				nodeID := node.ID()
				// Use the original graph to get the node name
				if namedNode, ok := g.Node(nodeID).(*NamedNode); ok {
					fmt.Printf("  - %s\n", namedNode.Name)
				} else {
					fmt.Printf("  - Node ID: %d\n", nodeID)
				}
			}
		}
	} else {
		fmt.Println("The graph remains fully connected after removing node 0.")
	}
}

func findUnderconstrainedSignals(graph *simple.UndirectedGraph) []string {
	underconstrained := []string{}
	nodes := graph.Nodes()
	for nodes.Next() {
		n := nodes.Node().(*NamedNode)
		if graph.From(n.ID()).Len() <= 1 {
			underconstrained = append(underconstrained, n.Name)
		}
	}
	return underconstrained
}
