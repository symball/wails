package commands

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/atterpac/refresh/engine"
	"github.com/atterpac/refresh/process"
	"gopkg.in/yaml.v3"
)

func ensureIgnored(list *[]string, pattern string) {
	for _, item := range *list {
		if item == pattern {
			return
		}
	}
	*list = append(*list, pattern)
}

func ensurePrimaryExitPolicy(executes []process.Execute) {
	for index := range executes {
		execute := &executes[index]
		if execute.Type == process.Primary && execute.ExitPolicy == "" {
			execute.ExitPolicy = process.ExitPolicyShutdown
		}
	}
}

func isInterruptError(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && isInterruptProcessState(exitErr.ProcessState)
}

type WatcherOptions struct {
	AppArgs string `name:"appargs" description:"Extra app arguments"`
	Config  string `description:"The config file including path" default:"."`
}

func Watcher(options *WatcherOptions) error {
	// Parse the config file
	type devConfig struct {
		Config engine.Config `yaml:"dev_mode"`
	}

	var devconfig devConfig

	// Parse the config file
	c, err := os.ReadFile(options.Config)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(c, &devconfig)
	if err != nil {
		return err
	}

	ensureIgnored(&devconfig.Config.Ignore.File, "*_test.go")
	ensurePrimaryExitPolicy(devconfig.Config.ExecStruct)
	if err := applyFrontendReadiness(&devconfig.Config, os.Getenv("FRONTEND_DEVSERVER_URL")); err != nil {
		return err
	}

	if options.AppArgs != "" {
		for idx, execAction := range devconfig.Config.ExecStruct {
			if execAction.Cmd == "wails3 task run" {
				clonedExec := execAction

				escapedAppArgs := options.AppArgs
				if runtime.GOOS == "windows" {
					escapedAppArgs = fmt.Sprintf("%q", options.AppArgs)
				} else {
					escapedAppArgs =
						"'" + strings.ReplaceAll(options.AppArgs, "'", `'"'"'`) + "'"
				}

				clonedExec.Cmd = fmt.Sprintf(
					"wails3 task run -appargs=%s",
					escapedAppArgs,
				)
				devconfig.Config.ExecStruct[idx] = clonedExec
				fmt.Printf("Executing %s\n", clonedExec.Cmd)
			}
		}
	}

	watcherEngine, err := engine.NewEngineFromConfig(devconfig.Config)
	if err != nil {
		return err
	}
	if err := watcherEngine.Start(); err != nil {
		if isInterruptError(err) {
			slog.Warn("graceful exit requested", "signal", os.Interrupt)
			return nil
		}
		return err
	}
	return nil
}

// Standard projects get startup ordering without rewriting their Taskfiles.
// Custom commands and explicitly configured readiness remain user-owned.
func applyFrontendReadiness(config *engine.Config, frontendURL string) error {
	if frontendURL == "" {
		return nil
	}
	for i := range config.ExecStruct {
		step := &config.ExecStruct[i]
		if step.Type != process.Background || step.Readiness != nil || len(step.Command) != 0 || strings.Join(strings.Fields(step.Cmd), " ") != "wails3 task common:dev:frontend" {
			continue
		}
		step.Readiness = &process.Readiness{HTTP: frontendURL, Timeout: "60s"}
		if err := step.Validate(); err != nil {
			return fmt.Errorf("invalid frontend readiness: %w", err)
		}
	}
	return nil
}
