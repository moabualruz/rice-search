package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/ricesearch/rice-search/internal/ml"
	"github.com/ricesearch/rice-search/internal/pkg/logger"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "rice-search",
		Short: "Rice Search - Intelligent code search platform",
		Long: `Rice Search is a pure Go code search platform with hybrid search,
neural reranking, and event-driven microservices architecture.

Run 'rice-search serve' to start the monolith server.
Run 'rice-search --help' for available commands.`,
		SilenceUsage: true,
	}

	// Global flags
	rootCmd.PersistentFlags().StringP("config", "c", "", "config file path")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().String("format", "text", "output format (text, json)")

	// Add subcommands
	rootCmd.AddCommand(
		serveCmd(),
		versionCmd(),
		modelsCmd(),
		// TODO: Add these as they're implemented
		// apiCmd(),
		// mlCmd(),
		// searchCmd(),
		// webCmd(),
		// indexCmd(),
		// queryCmd(),
		// storesCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func serveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the Rice Search server (monolith mode)",
		Long: `Start all services in a single process:
- API server (HTTP gateway)
- ML service (embeddings, reranking)
- Search service (Qdrant queries)
- Web UI (optional)

Services communicate via in-memory Go channels for minimal latency.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			port, _ := cmd.Flags().GetInt("port")
			fmt.Printf("Starting Rice Search server on port %d...\n", port)
			fmt.Println("TODO: Implement server startup")
			return nil
		},
	}

	cmd.Flags().IntP("port", "p", 8080, "HTTP server port")
	cmd.Flags().String("host", "0.0.0.0", "HTTP server host")
	cmd.Flags().Bool("no-web", false, "disable web UI")
	cmd.Flags().String("ml-url", "", "external ML service URL (skip embedded)")
	cmd.Flags().String("bus", "memory", "event bus type (memory, kafka, nats, redis)")

	return cmd
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("rice-search %s\n", version)
			fmt.Printf("  commit: %s\n", commit)
			fmt.Printf("  built:  %s\n", date)
		},
	}
}

func modelsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "models",
		Short: "Manage ML models",
		Long:  "Download, list, and verify ML models required for Rice Search.",
	}

	// Shared flags
	var modelsDir string
	cmd.PersistentFlags().StringVar(&modelsDir, "models-dir", "./models", "models directory")

	cmd.AddCommand(
		modelsListCmd(&modelsDir),
		modelsDownloadCmd(&modelsDir),
		modelsCheckCmd(&modelsDir),
	)

	return cmd
}

func modelsListCmd(modelsDir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available models",
		RunE: func(cmd *cobra.Command, args []string) error {
			log := logger.Default()
			mgr := ml.NewModelManager(*modelsDir, log)

			format, _ := cmd.Flags().GetString("format")
			if format == "json" {
				models := mgr.ListModels()
				data, _ := json.MarshalIndent(models, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			// Table format
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tTYPE\tSIZE\tDESCRIPTION")
			fmt.Fprintln(w, "----\t----\t----\t-----------")

			for _, model := range mgr.ListModels() {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					model.Name, model.Type, model.Size, model.Description)
			}

			w.Flush()
			return nil
		},
	}
}

func modelsDownloadCmd(modelsDir *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "download [model...]",
		Short: "Download models from HuggingFace",
		Long: `Download ML models from HuggingFace.

Without arguments, downloads all required models.
With arguments, downloads only the specified models.

Examples:
  rice-search models download                    # Download all models
  rice-search models download jina-embeddings-v3 # Download specific model`,
		RunE: func(cmd *cobra.Command, args []string) error {
			log := logger.New("info", "text")
			mgr := ml.NewModelManager(*modelsDir, log)

			// Ensure models directory exists
			absPath, _ := filepath.Abs(*modelsDir)
			fmt.Printf("Models directory: %s\n\n", absPath)

			if err := os.MkdirAll(*modelsDir, 0755); err != nil {
				return fmt.Errorf("failed to create models directory: %w", err)
			}

			// Progress callback
			progress := func(p ml.DownloadProgress) {
				if p.Complete {
					fmt.Printf("✓ %s: download complete\n", p.Model)
				} else if p.Error != "" {
					fmt.Printf("✗ %s: %s\n", p.Model, p.Error)
				} else if p.Total > 0 {
					fmt.Printf("  %s/%s: %.1f%% (%d/%d bytes)\r",
						p.Model, p.File, p.Percent, p.Downloaded, p.Total)
				}
			}

			if len(args) == 0 {
				// Download all models
				fmt.Println("Downloading all required models...")
				if err := mgr.DownloadAllModels(progress); err != nil {
					return err
				}
			} else {
				// Download specific models
				for _, name := range args {
					fmt.Printf("Downloading %s...\n", name)
					if err := mgr.DownloadModel(name, progress); err != nil {
						return err
					}
				}
			}

			fmt.Println("\nDone!")
			return nil
		},
	}

	return cmd
}

func modelsCheckCmd(modelsDir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Check installed models",
		RunE: func(cmd *cobra.Command, args []string) error {
			log := logger.Default()
			mgr := ml.NewModelManager(*modelsDir, log)

			format, _ := cmd.Flags().GetString("format")
			statuses := mgr.CheckAllModels()

			if format == "json" {
				data, _ := json.MarshalIndent(statuses, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			// Table format
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "MODEL\tSTATUS\tMISSING")
			fmt.Fprintln(w, "-----\t------\t-------")

			allInstalled := true
			for _, status := range statuses {
				statusStr := "✓ installed"
				missing := "-"

				if !status.Installed {
					statusStr = "✗ missing"
					allInstalled = false
					if len(status.Missing) > 0 {
						missing = fmt.Sprintf("%v", status.Missing)
					}
				}

				fmt.Fprintf(w, "%s\t%s\t%s\n", status.Name, statusStr, missing)
			}

			w.Flush()

			if !allInstalled {
				fmt.Println("\nRun 'rice-search models download' to download missing models.")
			}

			return nil
		},
	}
}
