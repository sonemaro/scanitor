package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

var (
	// These variables are set during build time
	Version     = "dev"
	BuildNumber = "unknown"
	BuildDate   = "unknown"
	GitCommit   = "unknown"
	GitBranch   = "unknown"
)

// BuildInfo contains all build and runtime information
type BuildInfo struct {
	// Version Information
	Version     string `json:"version"`
	SemVer      string `json:"semver"`
	BuildNumber string `json:"build_number"`
	BuildDate   string `json:"build_date"`

	// Git Information
	GitCommit string `json:"git_commit"`
	GitBranch string `json:"git_branch"`

	// Go Build Information
	GoVersion string `json:"go_version"`
	Compiler  string `json:"compiler"`
	Platform  string `json:"platform"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`

	// Runtime Information
	NumCPU     int  `json:"num_cpu"`
	GOMAXPROCS int  `json:"gomaxprocs"`
	CGOEnabled bool `json:"cgo_enabled"`

	// Memory Information
	MemStats runtime.MemStats `json:"mem_stats"`

	// Build Settings
	BuildTags []string `json:"build_tags"`
	BuildDeps []Module `json:"build_deps"`

	// Additional Runtime Details
	GoRoot string `json:"go_root"`
	GoPath string `json:"go_path"`
}

// Module represents a Go module dependency
type Module struct {
	Path    string `json:"path"`
	Version string `json:"version"`
	Sum     string `json:"sum"`
}

// GetBuildInfo returns comprehensive build information
func GetBuildInfo() BuildInfo {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	buildInfo, _ := debug.ReadBuildInfo()

	// Extract build tags and dependencies
	var buildTags []string
	var buildDeps []Module

	if buildInfo != nil {
		// Extract build settings
		for _, setting := range buildInfo.Settings {
			if setting.Key == "-tags" {
				buildTags = strings.Split(setting.Value, ",")
			}
		}

		// Extract dependencies
		for _, dep := range buildInfo.Deps {
			buildDeps = append(buildDeps, Module{
				Path:    dep.Path,
				Version: dep.Version,
				Sum:     dep.Sum,
			})
		}
	}

	return BuildInfo{
		// Version Information
		Version:     Version,
		SemVer:      strings.Split(Version, "-")[0],
		BuildNumber: BuildNumber,
		BuildDate:   BuildDate,

		// Git Information
		GitCommit: GitCommit,
		GitBranch: GitBranch,

		// Go Build Information
		GoVersion: runtime.Version(),
		Compiler:  runtime.Compiler,
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,

		// Runtime Information
		NumCPU:     runtime.NumCPU(),
		GOMAXPROCS: runtime.GOMAXPROCS(0),
		CGOEnabled: runtime.Compiler == "gc" && runtime.GOOS != "js" && runtime.GOOS != "wasm",

		// Memory Information
		MemStats: memStats,

		// Build Settings
		BuildTags: buildTags,
		BuildDeps: buildDeps,

		// Additional Runtime Details
		GoRoot: runtime.GOROOT(),
		GoPath: getGoPath(),
	}
}

// FullVersion returns a formatted string with complete version information
func FullVersion() string {
	info := GetBuildInfo()

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Scanitor %s\n", info.Version))
	b.WriteString("========================================\n\n")

	// Version Information
	b.WriteString("Version Information:\n")
	b.WriteString(fmt.Sprintf("  Version:      %s\n", info.Version))
	b.WriteString(fmt.Sprintf("  Semantic Ver: %s\n", info.SemVer))
	b.WriteString(fmt.Sprintf("  Build Number: %s\n", info.BuildNumber))
	b.WriteString(fmt.Sprintf("  Build Date:   %s\n", info.BuildDate))
	b.WriteString("\n")

	// Git Information
	b.WriteString("Git Information:\n")
	b.WriteString(fmt.Sprintf("  Commit:       %s\n", info.GitCommit))
	b.WriteString(fmt.Sprintf("  Branch:       %s\n", info.GitBranch))
	b.WriteString("\n")

	// Go Build Information
	b.WriteString("Go Build Information:\n")
	b.WriteString(fmt.Sprintf("  Go Version:   %s\n", info.GoVersion))
	b.WriteString(fmt.Sprintf("  Compiler:     %s\n", info.Compiler))
	b.WriteString(fmt.Sprintf("  Platform:     %s\n", info.Platform))
	b.WriteString(fmt.Sprintf("  CGO Enabled:  %t\n", info.CGOEnabled))
	b.WriteString("\n")

	// Runtime Information
	b.WriteString("Runtime Information:\n")
	b.WriteString(fmt.Sprintf("  OS:           %s\n", info.OS))
	b.WriteString(fmt.Sprintf("  Architecture: %s\n", info.Arch))
	b.WriteString(fmt.Sprintf("  CPUs:         %d\n", info.NumCPU))
	b.WriteString(fmt.Sprintf("  GOMAXPROCS:   %d\n", info.GOMAXPROCS))
	b.WriteString("\n")

	// Memory Information
	b.WriteString("Memory Information:\n")
	b.WriteString(fmt.Sprintf("  Alloc:        %d MB\n", info.MemStats.Alloc/1024/1024))
	b.WriteString(fmt.Sprintf("  Total Alloc:  %d MB\n", info.MemStats.TotalAlloc/1024/1024))
	b.WriteString(fmt.Sprintf("  Sys:          %d MB\n", info.MemStats.Sys/1024/1024))
	b.WriteString(fmt.Sprintf("  NumGC:        %d\n", info.MemStats.NumGC))
	b.WriteString("\n")

	// Build Settings
	if len(info.BuildTags) > 0 {
		b.WriteString("Build Tags:\n")
		for _, tag := range info.BuildTags {
			b.WriteString(fmt.Sprintf("  - %s\n", tag))
		}
		b.WriteString("\n")
	}

	// Environment
	b.WriteString("Go Environment:\n")
	b.WriteString(fmt.Sprintf("  GOROOT:       %s\n", info.GoRoot))
	b.WriteString(fmt.Sprintf("  GOPATH:       %s\n", info.GoPath))
	b.WriteString("\n")

	// Dependencies
	if len(info.BuildDeps) > 0 {
		b.WriteString("Dependencies:\n")
		for _, dep := range info.BuildDeps[:minInt(5, len(info.BuildDeps))] { // Show only first 5 deps
			b.WriteString(fmt.Sprintf("  - %s@%s\n", dep.Path, dep.Version))
		}
		if len(info.BuildDeps) > 5 {
			b.WriteString("  ... and more\n")
		}
	}

	return b.String()
}

// Helper functions
func getGoPath() string {
	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range buildInfo.Settings {
			if setting.Key == "GOPATH" {
				return setting.Value
			}
		}
	}
	return "unknown"
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
