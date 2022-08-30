package main

import (
	"context"
	"fmt"
	"os"
	"runtime/pprof"

	"github.com/bonnefoa/kubectl-fzf/v3/internal/completion"
	"github.com/bonnefoa/kubectl-fzf/v3/internal/fetcher"
	"github.com/bonnefoa/kubectl-fzf/v3/internal/fzf"
	"github.com/bonnefoa/kubectl-fzf/v3/internal/k8s/resources"
	"github.com/bonnefoa/kubectl-fzf/v3/internal/k8s/store"
	"github.com/bonnefoa/kubectl-fzf/v3/internal/parse"
	"github.com/bonnefoa/kubectl-fzf/v3/internal/results"
	"github.com/bonnefoa/kubectl-fzf/v3/internal/util"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	version   = "dev"
	gitCommit = "none"
	gitBranch = "unknown"
	goVersion = "unknown"
	buildDate = "unknown"
)

func versionFun(cmd *cobra.Command, args []string) {
	fmt.Printf("Version: %s\n", version)
	fmt.Printf("Git hash: %s\n", gitCommit)
	fmt.Printf("Git branch: %s\n", gitBranch)
	fmt.Printf("Build date: %s\n", buildDate)
	fmt.Printf("Go Version: %s\n", goVersion)
	os.Exit(0)
}

func completeFun(cmd *cobra.Command, args []string) {
	fetchConfigCli := fetcher.GetFetchConfigCli()
	f := fetcher.NewFetcher(&fetchConfigCli)
	err := f.LoadFetcherState()
	if err != nil {
		logrus.Warnf("Error loading fetcher state")
		os.Exit(6)
	}

	completionResults, err := completion.ProcessCommandArgs(cmd.Use, args, f)
	if e, ok := err.(resources.UnknownResourceError); ok {
		logrus.Warnf("Unknown resource type: %s", e)
		os.Exit(6)
	} else if e, ok := err.(parse.UnmanagedFlagError); ok {
		logrus.Warnf("Unmanaged flag: %s", e)
		os.Exit(6)
	} else if err != nil {
		logrus.Warnf("Error during completion: %s", err)
		os.Exit(6)
	}

	err = f.SaveFetcherState()
	if err != nil {
		logrus.Warnf("Error saving fetcher state: %s", err)
		os.Exit(6)
	}

	if err != nil {
		logrus.Fatalf("Completion error: %s", err)
	}
	if len(completionResults.Completions) == 0 {
		logrus.Warn("No completion found")
		os.Exit(5)
	}
	formattedComps := completionResults.GetFormattedOutput()

	// TODO pass query
	fzfResult, err := fzf.CallFzf(formattedComps, "")
	if err != nil {
		logrus.Fatalf("Call fzf error: %s", err)
	}
	res, err := results.ProcessResult(cmd.Use, args, f, fzfResult)
	if err != nil {
		logrus.Fatalf("Process result error: %s", err)
	}
	fmt.Print(res)
}

func statsFun(cmd *cobra.Command, args []string) {
	fetchConfigCli := fetcher.GetFetchConfigCli()
	f := fetcher.NewFetcher(&fetchConfigCli)
	ctx := context.Background()
	stats, err := f.GetStats(ctx)
	util.FatalIf(err)
	statsOutput := store.GetStatsOutput(stats)
	fmt.Print(statsOutput)
}

func addK8sCmd(rootCmd *cobra.Command) {
	var k8sCmd = &cobra.Command{
		Use:     "k8s_completion",
		Short:   "Subcommand grouping completion for kubectl cli verbs",
		Example: "kubectl-fzf-completion k8s_completion get pods \"\"",
	}
	rootCmd.AddCommand(k8sCmd)
	verbs := []string{"get", "exec", "logs", "label", "describe", "delete", "annotate", "edit"}
	for _, verb := range verbs {
		cmd := &cobra.Command{
			Use:                verb,
			Run:                completeFun,
			DisableFlagParsing: true,
			FParseErrWhitelist: cobra.FParseErrWhitelist{
				UnknownFlags: true,
			},
		}
		k8sCmd.AddCommand(cmd)
	}
}

func addStatsCmd(rootCmd *cobra.Command) {
	statsCmd := &cobra.Command{
		Use: "stats",
		Run: statsFun,
	}
	statsFlags := statsCmd.Flags()
	fetcher.SetFetchConfigFlags(statsFlags)
	err := viper.BindPFlags(statsFlags)
	util.FatalIf(err)
	rootCmd.AddCommand(statsCmd)
}

func main() {
	var rootCmd = &cobra.Command{
		Use: "kubectl_fzf_completion",
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
	}
	rootFlags := rootCmd.PersistentFlags()
	util.SetCommonCliFlags(rootFlags, "error")
	err := viper.BindPFlags(rootFlags)
	util.FatalIf(err)

	versionCmd := &cobra.Command{
		Use:   "version",
		Run:   versionFun,
		Short: "Print command version",
	}
	rootCmd.AddCommand(versionCmd)

	addK8sCmd(rootCmd)
	addStatsCmd(rootCmd)

	util.ConfigureViper()
	cobra.OnInitialize(util.CommonInitialization)
	defer pprof.StopCPUProfile()
	if err := rootCmd.Execute(); err != nil {
		logrus.Fatalf("Root command failed: %v", err)
	}
}
