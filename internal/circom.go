package internal

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func CheckCircomInstallation() error {
	cmd := exec.Command("circom", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("circom is not installed or not in PATH")
	}
	return nil
}

func GetCircomFiles(path string) ([]string, error) {
	var files []string

	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".circom") {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}

func CreateTempCircomFile(originalPath string) (string, error) {
	content, err := os.ReadFile(originalPath)
	if err != nil {
		return "", err
	}

	// Remove existing main component
	re := regexp.MustCompile(`(?m)^component\s+main\s*=.*$`)
	content = re.ReplaceAll(content, []byte{})

	originalDir := filepath.Dir(originalPath)

	tempFile, err := os.CreateTemp(originalDir, "circom_*.circom")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	if _, err := tempFile.Write(content); err != nil {
		return "", err
	}

	return tempFile.Name(), nil
}

func AddMainComponent(tempFilePath, templateName string, args []int) error {
	f, err := os.OpenFile(tempFilePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	mainComponent := fmt.Sprintf("\ncomponent main = %s(%s);\n", templateName, joinInts(args))
	if _, err := f.WriteString(mainComponent); err != nil {
		return err
	}

	return nil
}

func CompileCircuit(tempFilePath string) (string, string, error) {
	outputPath := strings.TrimSuffix(tempFilePath, filepath.Ext(tempFilePath))
	cmd := exec.Command("circom", "--json", "--sym", "--O0", "-o", filepath.Dir(tempFilePath), tempFilePath)
	err := cmd.Run()
	if err != nil {
		return "", "", fmt.Errorf("compilation failed: %v", err)
	}

	constraintsFile := outputPath + "_constraints.json"
	if _, err := os.Stat(constraintsFile); os.IsNotExist(err) {
		return "", "", fmt.Errorf("constraints file not generated")
	}

	symFile := outputPath + ".sym"
	if _, err := os.Stat(symFile); os.IsNotExist(err) {
		return "", "", fmt.Errorf("sym file not generated")
	}

	return constraintsFile, symFile, nil
}

func joinInts(ints []int) string {
	strInts := make([]string, len(ints))
	for i, v := range ints {
		strInts[i] = fmt.Sprintf("%d", v)
	}
	return strings.Join(strInts, ", ")
}

func GenerateRandomArgs(count int) []int {
	args := make([]int, count)
	for i := range args {
		args[i] = rand.Intn(14) + 2 // Random int in range [2, 15]
	}
	return args
}

// Each constraint is an array of three linear expressions. Each expression contains the signals used.
type Constraints [][3][]int64

func LoadFromJson(constraintsFile string) (Constraints, error) {
	// Variable to hold the unmarshaled data
	var constraints Constraints

	// Read the entire JSON file
	data, err := os.ReadFile(constraintsFile)
	if err != nil {
		return constraints, err
	}

	// Temp variable to hold the unmarshaled data
	var tempData struct {
		Constraints [][3]map[string]string `json:"constraints"`
	}
	err = json.Unmarshal(data, &tempData)
	if err != nil {
		return constraints, err
	}

	// Convert keys from string to integers
	for _, tempConstraint := range tempData.Constraints {
		var intConstraints [3][]int64
		for i, linearExpression := range tempConstraint {
			for key := range linearExpression {
				intKey := stringToInt(key)
				intConstraints[i] = append(intConstraints[i], intKey)
			}
		}
		constraints = append(constraints, intConstraints)
	}

	return constraints, nil
}

func LoadFromSym(symFile string) ([]string, error) {
	var signals []string

	// Open the file
	file, err := os.Open(symFile)
	if err != nil {
		return signals, err
	}
	defer file.Close()

	// Create a new CSV reader
	reader := csv.NewReader(file)
	reader.Comma = ',' // Set the delimiter to a comma (default)

	// Read all lines
	records, err := reader.ReadAll()
	if err != nil {
		return signals, err
	}

	// Ensure index 0 has "1"
	signals = append(signals, "1")

	// Loop through each record and extract the name (4th column)
	for _, record := range records {
		name := record[3] // The 'name' field is the 4th column (index 3)
		signals = append(signals, name)
	}

	return signals, nil
}

func stringToInt(s string) int64 {
	var result int64
	fmt.Sscanf(s, "%d", &result)
	return result
}
