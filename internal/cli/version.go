package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"

	"github.com/tdeshazo/repoctx/pkg/agentctx"
	"github.com/tdeshazo/repoctx/pkg/artifacts"
	"github.com/tdeshazo/repoctx/pkg/compiler"
	"github.com/tdeshazo/repoctx/pkg/discovery"
)

const buildInfoVersion = "repoctx.build/v1alpha1"

// These defaults describe an ordinary local Go build. Release tooling may set
// them with -ldflags -X; the values remain descriptive and unauthenticated.
var (
	buildRelease      = "devel"
	buildRevision     string
	buildModified     string
	buildDistribution = "go"
)

type contractVersions struct {
	IR        string `json:"ir"`
	Context   string `json:"context"`
	Discovery string `json:"discovery"`
	Artifacts string `json:"artifacts"`
}

type buildInfo struct {
	Version      string           `json:"version"`
	Program      string           `json:"program"`
	Release      string           `json:"release"`
	Revision     string           `json:"revision"`
	Modified     string           `json:"modified"`
	Distribution string           `json:"distribution"`
	GoVersion    string           `json:"go_version"`
	Platform     string           `json:"platform"`
	Contracts    contractVersions `json:"contracts"`
}

func versionCmd(args []string) {
	fs := flag.NewFlagSet("version", flag.ExitOnError)
	format := fs.String("format", "text", "output format: text or json")
	_ = fs.Parse(args)
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: repoctx version [-format text|json]")
		os.Exit(2)
	}

	info := currentBuildInfo()
	switch *format {
	case "text":
		writeBuildInfoText(info)
	case "json":
		if err := json.NewEncoder(os.Stdout).Encode(info); err != nil {
			fatal(err)
		}
	default:
		fmt.Fprintln(os.Stderr, "repoctx: version format must be text or json")
		os.Exit(2)
	}
}

func currentBuildInfo() buildInfo {
	release := buildRelease
	revision := buildRevision
	modified := normalizedModified(buildModified)
	if info, ok := debug.ReadBuildInfo(); ok {
		if release == "devel" && info.Main.Version != "" && info.Main.Version != "(devel)" {
			release = info.Main.Version
		}
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				if revision == "" {
					revision = setting.Value
				}
			case "vcs.modified":
				if modified == "unknown" {
					modified = normalizedModified(setting.Value)
				}
			}
		}
	}
	if revision == "" {
		revision = "unknown"
	}
	return buildInfo{
		Version:      buildInfoVersion,
		Program:      "repoctx",
		Release:      release,
		Revision:     revision,
		Modified:     modified,
		Distribution: buildDistribution,
		GoVersion:    runtime.Version(),
		Platform:     runtime.GOOS + "/" + runtime.GOARCH,
		Contracts: contractVersions{
			IR:        compiler.IRVersion,
			Context:   agentctx.Version,
			Discovery: discovery.Version,
			Artifacts: artifacts.Version,
		},
	}
}

func normalizedModified(value string) string {
	if value == "true" || value == "false" {
		return value
	}
	return "unknown"
}

func writeBuildInfoText(info buildInfo) {
	fmt.Printf("repoctx %s\n", info.Release)
	fmt.Printf("revision: %s\n", info.Revision)
	fmt.Printf("modified: %s\n", info.Modified)
	fmt.Printf("distribution: %s\n", info.Distribution)
	fmt.Printf("go: %s\n", info.GoVersion)
	fmt.Printf("platform: %s\n", info.Platform)
	fmt.Printf(
		"contracts: ir=%s context=%s discovery=%s artifacts=%s\n",
		info.Contracts.IR,
		info.Contracts.Context,
		info.Contracts.Discovery,
		info.Contracts.Artifacts,
	)
}
