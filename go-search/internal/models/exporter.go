// Package models provides ML model management and mapping for Rice Search.
package models

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ONNXExporter handles exporting HuggingFace models to ONNX format.
type ONNXExporter struct {
	optimumPath string // Path to optimum-cli executable
	modelsDir   string // Directory to store exported models
}

// NewONNXExporter creates a new ONNX exporter.
func NewONNXExporter(modelsDir string) *ONNXExporter {
	return &ONNXExporter{
		modelsDir: modelsDir,
	}
}

// IsOptimumAvailable checks if optimum-cli is installed and available.
func (e *ONNXExporter) IsOptimumAvailable() bool {
	path, err := e.findOptimumCLI()
	if err != nil {
		return false
	}
	e.optimumPath = path
	return true
}

// findOptimumCLI finds the optimum-cli executable.
func (e *ONNXExporter) findOptimumCLI() (string, error) {
	// Try common names
	names := []string{"optimum-cli"}
	if runtime.GOOS == "windows" {
		names = append(names, "optimum-cli.exe")
	}

	for _, name := range names {
		path, err := exec.LookPath(name)
		if err == nil {
			return path, nil
		}
	}

	// Try Python module
	pythonNames := []string{"python3", "python"}
	if runtime.GOOS == "windows" {
		pythonNames = []string{"python", "python3", "py"}
	}

	for _, python := range pythonNames {
		path, err := exec.LookPath(python)
		if err != nil {
			continue
		}

		// Check if optimum is installed as a module
		cmd := exec.Command(path, "-m", "optimum.exporters.onnx", "--help")
		if err := cmd.Run(); err == nil {
			return fmt.Sprintf("%s -m optimum.exporters.onnx", path), nil
		}
	}

	return "", fmt.Errorf("optimum-cli not found")
}

// GetInstallInstructions returns instructions for installing optimum.
func (e *ONNXExporter) GetInstallInstructions() string {
	return `To export HuggingFace models to ONNX, install optimum:

  pip install optimum[onnx]

Then you can export models using:

  optimum-cli export onnx --model <model_id> <output_dir>

Or use this tool to export automatically.`
}

// ExportProgress represents the progress of an export operation.
type ExportProgress struct {
	ModelID  string
	Status   string // "starting", "downloading", "exporting", "validating", "complete", "error"
	Message  string
	Percent  float64
	Complete bool
	Error    string
}

// ExportModel exports a HuggingFace model to ONNX format.
// Returns a channel that sends progress updates.
func (e *ONNXExporter) ExportModel(ctx context.Context, modelID string, task string) (<-chan ExportProgress, error) {
	if !e.IsOptimumAvailable() {
		return nil, fmt.Errorf("optimum-cli not available: %s", e.GetInstallInstructions())
	}

	progressChan := make(chan ExportProgress, 10)

	go func() {
		defer close(progressChan)

		// Create output directory
		outputDir := filepath.Join(e.modelsDir, modelID)
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			progressChan <- ExportProgress{
				ModelID: modelID,
				Status:  "error",
				Error:   fmt.Sprintf("failed to create output directory: %v", err),
			}
			return
		}

		progressChan <- ExportProgress{
			ModelID: modelID,
			Status:  "starting",
			Message: fmt.Sprintf("Starting export of %s", modelID),
			Percent: 0,
		}

		// Build command
		var cmd *exec.Cmd
		args := []string{"export", "onnx", "--model", modelID}
		if task != "" {
			args = append(args, "--task", task)
		}
		args = append(args, outputDir)

		if strings.Contains(e.optimumPath, " -m ") {
			// Python module invocation
			parts := strings.SplitN(e.optimumPath, " -m ", 2)
			pythonPath := parts[0]
			module := parts[1]
			cmdArgs := append([]string{"-m", module}, args...)
			cmd = exec.CommandContext(ctx, pythonPath, cmdArgs...)
		} else {
			cmd = exec.CommandContext(ctx, e.optimumPath, args...)
		}

		// Capture output for progress
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			progressChan <- ExportProgress{
				ModelID: modelID,
				Status:  "error",
				Error:   fmt.Sprintf("failed to create stdout pipe: %v", err),
			}
			return
		}

		stderr, err := cmd.StderrPipe()
		if err != nil {
			progressChan <- ExportProgress{
				ModelID: modelID,
				Status:  "error",
				Error:   fmt.Sprintf("failed to create stderr pipe: %v", err),
			}
			return
		}

		if err := cmd.Start(); err != nil {
			progressChan <- ExportProgress{
				ModelID: modelID,
				Status:  "error",
				Error:   fmt.Sprintf("failed to start export: %v", err),
			}
			return
		}

		// Read output and send progress
		go func() {
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				line := scanner.Text()
				status := "exporting"
				percent := 50.0

				if strings.Contains(line, "Downloading") {
					status = "downloading"
					percent = 25.0
				} else if strings.Contains(line, "Validating") {
					status = "validating"
					percent = 75.0
				} else if strings.Contains(line, "All good") || strings.Contains(line, "model saved") {
					status = "complete"
					percent = 100.0
				}

				progressChan <- ExportProgress{
					ModelID: modelID,
					Status:  status,
					Message: line,
					Percent: percent,
				}
			}
		}()

		// Capture stderr for errors
		var stderrOutput strings.Builder
		go func() {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				stderrOutput.WriteString(scanner.Text())
				stderrOutput.WriteString("\n")
			}
		}()

		// Wait for command to complete
		if err := cmd.Wait(); err != nil {
			errMsg := stderrOutput.String()
			if errMsg == "" {
				errMsg = err.Error()
			}
			progressChan <- ExportProgress{
				ModelID: modelID,
				Status:  "error",
				Error:   fmt.Sprintf("export failed: %s", errMsg),
			}
			return
		}

		// Verify model.onnx exists
		onnxPath := filepath.Join(outputDir, "model.onnx")
		if _, err := os.Stat(onnxPath); os.IsNotExist(err) {
			progressChan <- ExportProgress{
				ModelID: modelID,
				Status:  "error",
				Error:   "export completed but model.onnx not found",
			}
			return
		}

		progressChan <- ExportProgress{
			ModelID:  modelID,
			Status:   "complete",
			Message:  fmt.Sprintf("Successfully exported to %s", outputDir),
			Percent:  100,
			Complete: true,
		}
	}()

	return progressChan, nil
}

// GetTaskForModelType returns the appropriate optimum task for a model type.
func GetTaskForModelType(modelType ModelType) string {
	switch modelType {
	case ModelTypeEmbed:
		return "feature-extraction"
	case ModelTypeRerank:
		return "text-classification"
	case ModelTypeQueryUnderstand:
		return "text-classification"
	default:
		return "" // Let optimum auto-detect
	}
}

// ExportModelSync exports a model synchronously (blocking).
func (e *ONNXExporter) ExportModelSync(ctx context.Context, modelID string, task string) error {
	progressChan, err := e.ExportModel(ctx, modelID, task)
	if err != nil {
		return err
	}

	var lastError string
	for progress := range progressChan {
		if progress.Error != "" {
			lastError = progress.Error
		}
	}

	if lastError != "" {
		return fmt.Errorf("export failed: %s", lastError)
	}

	return nil
}
