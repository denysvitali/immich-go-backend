package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/denysvitali/immich-go-backend/internal/openapicoverage"
)

var (
	ocSpecFlag    string
	ocGenDirFlag  string
	ocMDFlag      string
	ocJSONFlag    string
	ocFailUnder   float64
	ocIgnore      []string
	ocVerboseFlag bool
)

var openapiCoverageCmd = &cobra.Command{
	Use:   "openapi-coverage",
	Short: "Compute OpenAPI vs gRPC-implementation coverage",
	Long: `openapi-coverage compares the routes declared in the upstream Immich
OpenAPI spec against the HTTP routes exposed by this Go backend via
its generated grpc-gateway files.

It exits with a non-zero status if the coverage falls below the
threshold passed via --fail-under (default: never).`,
	RunE: runOpenAPICoverage,
}

func init() {
	// Sensible defaults: the spec lives next to the project, the
	// generated gateway files live under internal/proto/gen.
	defaultSpec := defaultOpenAPISpecPath()
	defaultGen := defaultOpenAPIGenDir()

	openapiCoverageCmd.Flags().StringVar(&ocSpecFlag, "spec", defaultSpec,
		"path to immich-openapi-specs.json")
	openapiCoverageCmd.Flags().StringVar(&ocGenDirFlag, "gen-dir", defaultGen,
		"path to the generated proto directory (containing *.pb.gw.go files)")
	openapiCoverageCmd.Flags().StringVar(&ocMDFlag, "md", "",
		"if set, write a markdown report to this path")
	openapiCoverageCmd.Flags().StringVar(&ocJSONFlag, "json", "",
		"if set, write a JSON report to this path (otherwise: stdout)")
	openapiCoverageCmd.Flags().Float64Var(&ocFailUnder, "fail-under", 0,
		"exit non-zero if coverage < N percent (0 disables the check)")
	openapiCoverageCmd.Flags().StringSliceVar(&ocIgnore, "ignore-prefix", nil,
		"comma-separated list of path prefixes to ignore (e.g. /server)")
	openapiCoverageCmd.Flags().BoolVarP(&ocVerboseFlag, "verbose", "v", false,
		"print per-file progress to stderr")

	rootCmd.AddCommand(openapiCoverageCmd)
}

func defaultOpenAPISpecPath() string {
	// The spec is checked in at <repo>/immich-upstream/open-api/...
	// The binary may be run from any cwd, so resolve relative to the
	// executable's working directory first, then fall back to the
	// repo-relative path.
	candidates := []string{
		"immich-upstream/open-api/immich-openapi-specs.json",
		"../../immich-upstream/open-api/immich-openapi-specs.json",
		"../immich-upstream/open-api/immich-openapi-specs.json",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	return "immich-upstream/open-api/immich-openapi-specs.json"
}

func defaultOpenAPIGenDir() string {
	candidates := []string{
		"internal/proto/gen/immich/v1",
		"../../internal/proto/gen/immich/v1",
		"../internal/proto/gen/immich/v1",
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	return "internal/proto/gen/immich/v1"
}

func runOpenAPICoverage(cmd *cobra.Command, args []string) error {
	log := logrus.WithField("cmd", "openapi-coverage")

	if ocVerboseFlag {
		logrus.SetLevel(logrus.DebugLevel)
	}

	log.WithField("spec", ocSpecFlag).Info("parsing OpenAPI spec")
	upstream, err := openapicoverage.ParseOpenAPI(ocSpecFlag)
	if err != nil {
		return fmt.Errorf("parse OpenAPI spec: %w", err)
	}
	log.WithField("count", len(upstream)).Debug("upstream routes")

	log.WithField("dir", ocGenDirFlag).Info("parsing gateway files")
	gateway, err := openapicoverage.ParseGatewayDir(ocGenDirFlag)
	if err != nil {
		return fmt.Errorf("parse gateway dir: %w", err)
	}
	log.WithField("count", len(gateway)).Debug("gateway routes")

	report := openapicoverage.Diff(upstream, gateway, ocIgnore)

	// JSON to stdout (or to --json file).
	if ocJSONFlag == "" {
		if err := openapicoverage.WriteJSON(os.Stdout, report); err != nil {
			return fmt.Errorf("write json: %w", err)
		}
	} else {
		if err := openapicoverage.WriteJSONFile(ocJSONFlag, report); err != nil {
			return fmt.Errorf("write json file: %w", err)
		}
	}

	// Optional markdown file.
	if ocMDFlag != "" {
		if err := openapicoverage.WriteMarkdownFile(ocMDFlag, report); err != nil {
			return fmt.Errorf("write markdown: %w", err)
		}
		log.WithField("path", ocMDFlag).Info("wrote markdown report")
	}

	// Always print a colored one-line summary to stderr.
	fmt.Fprintf(os.Stderr,
		"openapi-coverage: %.2f%% (%d/%d implemented, %d missing, %d extra)\n",
		report.CoveragePct, report.Implemented, report.UpstreamTotal,
		report.Missing, report.ExtraImplemented,
	)

	if ocFailUnder > 0 && report.CoveragePct < ocFailUnder {
		return fmt.Errorf(
			"coverage %.2f%% is below threshold %.2f%%",
			report.CoveragePct, ocFailUnder,
		)
	}
	return nil
}
