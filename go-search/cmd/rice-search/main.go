// Package main provides the Rice Search CLI client.
// This client connects to rice-search-server via gRPC for all operations.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/ricesearch/rice-search/internal/grpcclient"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// Global client
var client *grpcclient.Client

func main() {
	rootCmd := &cobra.Command{
		Use:   "rice-search",
		Short: "Rice Search - Intelligent code search CLI",
		Long: `Rice Search CLI connects to a rice-search-server via gRPC for
intelligent code search, indexing, and store management.

Start the server first:
  rice-search-server

Then use this CLI:
  rice-search search "authentication handler"
  rice-search index ./src
  rice-search stores list`,
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip connection for version and help
			if cmd.Name() == "version" || cmd.Name() == "help" {
				return nil
			}
			return connectToServer(cmd)
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			if client != nil {
				if err := client.Close(); err != nil {
					// Log close error but don't fail - we're shutting down anyway
					_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to close client: %v\n", err)
				}
			}
		},
	}

	// Global flags
	rootCmd.PersistentFlags().StringP("server", "S", "auto", "server address (auto, localhost:50051, or unix:///path)")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().String("format", "text", "output format (text, json)")

	// Add subcommands
	rootCmd.AddCommand(
		versionCmd(),
		searchCmd(),
		indexCmd(),
		storesCmd(),
		healthCmd(),
		modelsCmd(),
		watchCmd,
	)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// connectToServer establishes connection to the server.
func connectToServer(cmd *cobra.Command) error {
	serverAddr, _ := cmd.Flags().GetString("server")

	cfg := grpcclient.DefaultConfig()
	cfg.ServerAddress = serverAddr

	var err error
	client, err = grpcclient.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w\n\nMake sure rice-search-server is running", err)
	}

	return nil
}

// =============================================================================
// Version Command
// =============================================================================

func versionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("rice-search %s (client)\n", version)
			fmt.Printf("  commit: %s\n", commit)
			fmt.Printf("  built:  %s\n", date)

			// Try to get server version
			if err := connectToServer(cmd); err == nil && client != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				if info, err := client.Version(ctx); err == nil {
					fmt.Printf("\nServer:\n")
					fmt.Printf("  version: %s\n", info.Version)
					fmt.Printf("  commit:  %s\n", info.Commit)
					fmt.Printf("  go:      %s\n", info.GoVersion)
				}
			}

			return nil
		},
	}
	return cmd
}

// =============================================================================
// Search Command
// =============================================================================

func searchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search indexed code",
		Long: `Search for code across indexed files.

Examples:
  rice-search search "authentication handler"
  rice-search search "error handling" -k 50
  rice-search search "database connection" -s myproject
  rice-search search "func main" --path-prefix cmd/`,
		Args: cobra.ExactArgs(1),
		RunE: runSearch,
	}

	cmd.Flags().StringP("store", "s", "default", "store to search")
	cmd.Flags().IntP("top-k", "k", 20, "number of results")
	cmd.Flags().Bool("no-rerank", false, "disable neural reranking")
	cmd.Flags().Bool("content", false, "include content in results")
	cmd.Flags().String("path-prefix", "", "filter by path prefix")
	cmd.Flags().StringSlice("lang", nil, "filter by language (e.g., go,typescript)")

	return cmd
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := args[0]
	storeName, _ := cmd.Flags().GetString("store")
	topK, _ := cmd.Flags().GetInt("top-k")
	noRerank, _ := cmd.Flags().GetBool("no-rerank")
	includeContent, _ := cmd.Flags().GetBool("content")
	pathPrefix, _ := cmd.Flags().GetString("path-prefix")
	languages, _ := cmd.Flags().GetStringSlice("lang")
	verbose, _ := cmd.Flags().GetBool("verbose")
	format, _ := cmd.Flags().GetString("format")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	opts := grpcclient.SearchOptions{
		TopK:           topK,
		IncludeContent: includeContent,
		PathPrefix:     pathPrefix,
		Languages:      languages,
	}

	if noRerank {
		f := false
		opts.EnableReranking = &f
	}

	resp, err := client.Search(ctx, storeName, query, opts)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if format == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	if len(resp.Results) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	fmt.Printf("Found %d results in %dms\n\n", resp.Total, resp.SearchTimeMs)

	for i, r := range resp.Results {
		scoreStr := fmt.Sprintf("%.3f", r.Score)
		if r.RerankScore != nil {
			scoreStr = fmt.Sprintf("%.3f (rerank: %.3f)", r.Score, *r.RerankScore)
		}
		fmt.Printf("[%d] %s:%d-%d  score: %s\n",
			i+1, r.Path, r.StartLine, r.EndLine, scoreStr)

		if len(r.Symbols) > 0 && verbose {
			fmt.Printf("    symbols: %s\n", strings.Join(r.Symbols, ", "))
		}

		if includeContent && r.Content != "" {
			lines := strings.Split(r.Content, "\n")
			preview := lines
			if len(preview) > 5 {
				preview = preview[:5]
			}
			for _, line := range preview {
				if len(line) > 100 {
					line = line[:100] + "..."
				}
				fmt.Printf("    │ %s\n", line)
			}
			if len(lines) > 5 {
				fmt.Printf("    │ ... (%d more lines)\n", len(lines)-5)
			}
		}

		fmt.Println()
	}

	if verbose {
		fmt.Printf("---\n")
		fmt.Printf("Timing: embed=%dms, retrieve=%dms",
			resp.EmbedTimeMs, resp.RetrievalTimeMs)
		if resp.RerankingApplied {
			fmt.Printf(", rerank=%dms (%d candidates)",
				resp.RerankTimeMs, resp.CandidatesReranked)
		}
		fmt.Println()
	}

	return nil
}

// =============================================================================
// Index Command
// =============================================================================

func indexCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "index <path>...",
		Short: "Index files into a store",
		Long: `Index source code files into a search store.

Examples:
  rice-search index ./src               # Index src directory into default store
  rice-search index ./src -s myproject  # Index into specific store
  rice-search index ./main.go ./lib     # Index specific files/directories`,
		Args: cobra.MinimumNArgs(1),
		RunE: runIndex,
	}

	cmd.Flags().StringP("store", "s", "default", "target store name")
	cmd.Flags().BoolP("force", "f", false, "force re-index unchanged files")
	cmd.Flags().StringSliceP("include", "i", nil, "include patterns (e.g., *.go,*.ts)")
	cmd.Flags().StringSliceP("exclude", "e", nil, "exclude patterns (e.g., vendor/*)")

	return cmd
}

func runIndex(cmd *cobra.Command, args []string) error {
	storeName, _ := cmd.Flags().GetString("store")
	force, _ := cmd.Flags().GetBool("force")
	include, _ := cmd.Flags().GetStringSlice("include")
	exclude, _ := cmd.Flags().GetStringSlice("exclude")
	format, _ := cmd.Flags().GetString("format")

	// Collect documents
	var docs []grpcclient.IndexDocument
	var totalSize int64

	for _, path := range args {
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("cannot access %s: %w", path, err)
		}

		if info.IsDir() {
			err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					name := d.Name()
					if name == ".git" || name == "node_modules" || name == "vendor" ||
						name == "__pycache__" || name == ".venv" || name == "target" {
						return filepath.SkipDir
					}
					return nil
				}

				if !shouldIncludeFile(p, include, exclude) {
					return nil
				}

				content, err := os.ReadFile(p)
				if err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "Warning: cannot read %s: %v\n", p, err)
					return nil
				}

				if isBinaryContent(content) {
					return nil
				}

				relPath, _ := filepath.Rel(".", p)
				docs = append(docs, grpcclient.IndexDocument{
					Path:    filepath.ToSlash(relPath),
					Content: string(content),
				})
				totalSize += int64(len(content))
				return nil
			})
			if err != nil {
				return fmt.Errorf("failed to walk %s: %w", path, err)
			}
		} else {
			content, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("cannot read %s: %w", path, err)
			}

			relPath, _ := filepath.Rel(".", path)
			docs = append(docs, grpcclient.IndexDocument{
				Path:    filepath.ToSlash(relPath),
				Content: string(content),
			})
			totalSize += int64(len(content))
		}
	}

	if len(docs) == 0 {
		fmt.Println("No files to index.")
		return nil
	}

	fmt.Printf("Indexing %d files (%.2f KB) into store '%s'...\n",
		len(docs), float64(totalSize)/1024, storeName)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	result, err := client.Index(ctx, storeName, docs, force)
	if err != nil {
		return fmt.Errorf("indexing failed: %w", err)
	}

	if format == "json" {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("✓ Indexed: %d, Skipped: %d, Failed: %d\n",
		result.Indexed, result.Skipped, result.Failed)
	fmt.Printf("  Chunks created: %d\n", result.ChunksTotal)
	fmt.Printf("  Duration: %s\n", result.Duration.Round(time.Millisecond))
	return nil
}

// =============================================================================
// Stores Commands
// =============================================================================

func storesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stores",
		Short: "Manage search stores",
		Long:  "Create, list, delete, and get statistics for search stores.",
	}

	cmd.AddCommand(
		storesListCmd(),
		storesCreateCmd(),
		storesDeleteCmd(),
		storesStatsCmd(),
	)

	return cmd
}

func storesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all stores",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			stores, err := client.ListStores(ctx)
			if err != nil {
				return fmt.Errorf("failed to list stores: %w", err)
			}

			format, _ := cmd.Flags().GetString("format")
			if format == "json" {
				data, _ := json.MarshalIndent(stores, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(stores) == 0 {
				fmt.Println("No stores found.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "NAME\tDESCRIPTION\tCREATED")
			_, _ = fmt.Fprintln(w, "----\t-----------\t-------")

			for _, s := range stores {
				desc := s.Description
				if desc == "" {
					desc = "-"
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", s.Name, desc, s.CreatedAt.Format(time.RFC3339))
			}

			_ = w.Flush()
			return nil
		},
	}
}

func storesCreateCmd() *cobra.Command {
	var description string

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new store",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			name := args[0]
			store, err := client.CreateStore(ctx, name, description)
			if err != nil {
				return fmt.Errorf("failed to create store: %w", err)
			}

			format, _ := cmd.Flags().GetString("format")
			if format == "json" {
				data, _ := json.MarshalIndent(store, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Printf("✓ Created store: %s\n", name)
			return nil
		},
	}

	cmd.Flags().StringVarP(&description, "description", "d", "", "store description")
	return cmd
}

func storesDeleteCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a store",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if !force {
				fmt.Printf("Delete store '%s'? This cannot be undone. [y/N]: ", name)
				var confirm string
				_, _ = fmt.Scanln(&confirm)
				if strings.ToLower(confirm) != "y" {
					fmt.Println("Cancelled.")
					return nil
				}
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := client.DeleteStore(ctx, name); err != nil {
				return fmt.Errorf("failed to delete store: %w", err)
			}

			fmt.Printf("✓ Deleted store: %s\n", name)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation")
	return cmd
}

func storesStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats <name>",
		Short: "Get store statistics",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			name := args[0]
			stats, err := client.GetStoreStats(ctx, name)
			if err != nil {
				return fmt.Errorf("failed to get store stats: %w", err)
			}

			format, _ := cmd.Flags().GetString("format")
			if format == "json" {
				data, _ := json.MarshalIndent(stats, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Printf("Store: %s\n", name)
			fmt.Printf("  Documents: %d\n", stats.DocumentCount)
			fmt.Printf("  Chunks:    %d\n", stats.ChunkCount)
			fmt.Printf("  Size:      %d bytes\n", stats.TotalSize)
			return nil
		},
	}
}

// =============================================================================
// Health Command
// =============================================================================

func healthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Check server health",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			health, err := client.Health(ctx)
			if err != nil {
				return fmt.Errorf("health check failed: %w", err)
			}

			format, _ := cmd.Flags().GetString("format")
			if format == "json" {
				data, _ := json.MarshalIndent(health, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			statusIcon := "✓"
			if health.Status == grpcclient.HealthStatusDegraded {
				statusIcon = "⚠"
			} else if health.Status == grpcclient.HealthStatusUnhealthy {
				statusIcon = "✗"
			}

			fmt.Printf("%s Server: %s (v%s)\n\n", statusIcon, health.Status, health.Version)
			fmt.Println("Components:")

			for name, comp := range health.Components {
				icon := "✓"
				if comp.Status == grpcclient.HealthStatusDegraded {
					icon = "⚠"
				} else if comp.Status == grpcclient.HealthStatusUnhealthy {
					icon = "✗"
				}
				fmt.Printf("  %s %s: %s", icon, name, comp.Message)
				if comp.Latency > 0 {
					fmt.Printf(" (%s)", comp.Latency.Round(time.Microsecond))
				}
				fmt.Println()
			}

			return nil
		},
	}
}

// =============================================================================
// Helpers
// =============================================================================

func shouldIncludeFile(path string, include, exclude []string) bool {
	defaultExts := []string{
		".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rs", ".java", ".kt",
		".c", ".cpp", ".h", ".hpp", ".cs", ".rb", ".php", ".swift", ".scala",
		".vue", ".svelte", ".md", ".yaml", ".yml", ".json", ".toml", ".sql",
	}

	ext := strings.ToLower(filepath.Ext(path))

	for _, pattern := range exclude {
		if matched, _ := filepath.Match(pattern, path); matched {
			return false
		}
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return false
		}
	}

	if len(include) > 0 {
		for _, pattern := range include {
			if matched, _ := filepath.Match(pattern, path); matched {
				return true
			}
			if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
				return true
			}
		}
		return false
	}

	for _, e := range defaultExts {
		if ext == e {
			return true
		}
	}

	return false
}

func isBinaryContent(content []byte) bool {
	checkLen := len(content)
	if checkLen > 8192 {
		checkLen = 8192
	}

	for i := 0; i < checkLen; i++ {
		if content[i] == 0 {
			return true
		}
	}
	return false
}

// =============================================================================
// Models Commands
// =============================================================================

func modelsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "models",
		Short: "Manage ML models",
		Long:  "Download, list, and check ML models used for embeddings, reranking, and query understanding.",
	}

	cmd.AddCommand(
		modelsListCmd(),
		modelsDownloadCmd(),
		modelsCheckCmd(),
	)

	return cmd
}

func modelsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available models",
		Long: `List all available ML models.

Examples:
  rice-search models list              # List all models
  rice-search models list --format json # JSON output`,
		RunE: runModelsList,
	}
	return cmd
}

func runModelsList(cmd *cobra.Command, args []string) error {
	format, _ := cmd.Flags().GetString("format")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	models, err := client.ListModels(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to list models: %w", err)
	}

	if format == "json" {
		data, _ := json.MarshalIndent(models, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Println("AVAILABLE MODELS:")
	fmt.Println()
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "TYPE\tNAME\tSIZE\tSTATUS\tDEFAULT\tGPU")
	_, _ = fmt.Fprintln(w, "----\t----\t----\t------\t-------\t---")

	for _, m := range models {
		status := "✗ not downloaded"
		if m.Downloaded {
			status = "✓ downloaded"
		}

		defaultMark := ""
		if m.IsDefault {
			defaultMark = "✓"
		}

		gpuMark := ""
		if m.GPUEnabled {
			gpuMark = "✓"
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			m.Type, m.DisplayName, formatBytes(m.Size), status, defaultMark, gpuMark)
	}
	_ = w.Flush()
	return nil
}

func modelsDownloadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "download [model-id]",
		Short: "Download ML models",
		Long: `Download one or all ML models.

Examples:
  rice-search models download                    # Download all models
  rice-search models download jina-embeddings-v2 # Download specific model`,
		RunE: runModelsDownload,
	}
	return cmd
}

func runModelsDownload(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// If no model ID provided, download all
	if len(args) == 0 {
		models, err := client.ListModels(ctx, "")
		if err != nil {
			return fmt.Errorf("failed to list models: %w", err)
		}

		fmt.Printf("Downloading %d models...\n\n", len(models))

		for _, m := range models {
			if m.Downloaded {
				fmt.Printf("✓ %s: already downloaded\n", m.DisplayName)
				continue
			}

			fmt.Printf("Downloading %s (%s)...\n", m.DisplayName, formatBytes(m.Size))
			result, err := client.DownloadModel(ctx, m.ID)
			if err != nil {
				fmt.Printf("✗ %s: %v\n", m.DisplayName, err)
				continue
			}

			if result.Success {
				fmt.Printf("✓ %s: downloaded\n", m.DisplayName)
			} else {
				fmt.Printf("✗ %s: %s\n", m.DisplayName, result.Message)
			}
		}

		fmt.Println("\nDone!")
		return nil
	}

	// Download specific model
	modelID := args[0]
	fmt.Printf("Downloading model: %s\n", modelID)

	result, err := client.DownloadModel(ctx, modelID)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	if result.Success {
		fmt.Printf("✓ Downloaded successfully\n")
	} else {
		fmt.Printf("✗ Download failed: %s\n", result.Message)
	}

	return nil
}

func modelsCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check installed models",
		Long: `Check which models are installed and their status.

Examples:
  rice-search models check`,
		RunE: runModelsCheck,
	}
	return cmd
}

func runModelsCheck(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	models, err := client.ListModels(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to list models: %w", err)
	}

	downloadedCount := 0
	totalSize := int64(0)

	fmt.Println("MODEL STATUS:")
	fmt.Println()

	for _, m := range models {
		if m.Downloaded {
			downloadedCount++
			totalSize += m.Size
			fmt.Printf("✓ %s (%s)\n", m.DisplayName, m.Type)
			if m.IsDefault {
				fmt.Printf("  Default model for %s\n", m.Type)
			}
			if m.GPUEnabled {
				fmt.Printf("  GPU acceleration: enabled\n")
			}
		} else {
			fmt.Printf("✗ %s (%s) - not downloaded\n", m.DisplayName, m.Type)
		}
	}

	fmt.Printf("\nTotal: %d/%d models downloaded (%s)\n",
		downloadedCount, len(models), formatBytes(totalSize))

	if downloadedCount < len(models) {
		fmt.Println("\nRun 'rice-search models download' to download missing models")
	}

	return nil
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
