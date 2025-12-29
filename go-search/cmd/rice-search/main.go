package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
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
		// TODO: Add these as they're implemented
		// apiCmd(),
		// mlCmd(),
		// searchCmd(),
		// webCmd(),
		// modelsCmd(),
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
