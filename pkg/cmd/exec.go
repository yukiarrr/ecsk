/*
Copyright © 2021 yukiarrr

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/spf13/cobra"
	"github.com/yukiarrr/ecsk/pkg/store"
	"github.com/yukiarrr/ecsk/pkg/ui"
	"github.com/yukiarrr/ecsk/pkg/util"
)

type ExecCommandOptions struct {
	Cluster            string
	Task               string
	Container          string
	Interactive        bool
	Plugin             string
	EnableErrorChecker bool
	Command            string
	Region             string
	Profile            string
}

//go:embed amazon-ecs-exec-checker/check-ecs-exec.sh
var checkScript []byte

func init() {
	var opts ExecCommandOptions

	execCmd := &cobra.Command{
		Use:   "exec",
		Short: `Execute commands like "docker exec"`,
		Long: `# ecsk exec -i -- [command]

After selecting the task and container interactively, and execute the command.`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithCancel(context.Background())
			util.HandleSignals(cancel)

			region, err := rootCmd.Flags().GetString("region")
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			profile, err := rootCmd.Flags().GetString("profile")
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			code, err := rootCmd.Flags().GetString("code")
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}

			cfg, err := store.NewConfig(ctx, region, profile, code)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}

			ecsClient := ecs.NewFromConfig(cfg)
			opts.Region = cfg.Region
			opts.Profile = profile

			argsLenAtDash := cmd.ArgsLenAtDash()
			if argsLenAtDash > -1 {
				opts.Command = strings.Join(args[argsLenAtDash:], " ")
			} else {
				fmt.Fprintln(os.Stderr, `Need command. Try "ecsk exec --help".`)
				os.Exit(1)
			}

			err = nextExecState(ctx, ecsClient, ui.Cluster, opts)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		},
	}

	rootCmd.AddCommand(execCmd)

	execCmd.Flags().StringVar(&opts.Cluster, "cluster", "", "The Amazon Resource Name (ARN) or short name of the cluster the task is running in. (From AWS CLI)")
	execCmd.Flags().StringVar(&opts.Task, "task", "", "The Amazon Resource Name (ARN) or ID of the task the container is part of. (From AWS CLI)")
	execCmd.Flags().StringVar(&opts.Container, "container", "", "The name of the container to execute the command on. (From AWS CLI)")
	execCmd.Flags().BoolVarP(&opts.Interactive, "interactive", "i", false, "Use this flag to run your command in interactive mode. (From AWS CLI)")
	execCmd.Flags().StringVar(&opts.Plugin, "plugin", "session-manager-plugin", "Path of session-manager-plugin.")
	execCmd.Flags().BoolVar(&opts.EnableErrorChecker, "enable-error-checker", true, "Whether to enable the error checker.")
}

func nextExecState(ctx context.Context, ecsClient *ecs.Client, state int, opts ExecCommandOptions) error {
	switch state {
	case ui.Cluster:
		if opts.Cluster != "" {
			return nextExecState(ctx, ecsClient, ui.Task, opts)
		}

		result, err := ui.AskCluster(ctx, ecsClient, false)
		if err != nil {
			return err
		}
		if result == "" {
			return errors.New("Canceled.")
		}

		opts.Cluster = result
		return nextExecState(ctx, ecsClient, ui.Task, opts)
	case ui.Task:
		if opts.Task != "" {
			return nextExecState(ctx, ecsClient, ui.Container, opts)
		}

		result, err := ui.AskTask(ctx, ecsClient, opts.Cluster, true)
		if err != nil {
			return err
		}
		if result == "" {
			opts.Cluster = ""
			opts.Task = ""
			return nextExecState(ctx, ecsClient, ui.Cluster, opts)
		}

		opts.Task = result
		return nextExecState(ctx, ecsClient, ui.Container, opts)
	case ui.Container:
		if opts.Container != "" {
			return nextExecState(ctx, ecsClient, ui.Complete, opts)
		}

		result, err := ui.AskContainer(ctx, ecsClient, opts.Cluster, opts.Task, true)
		if err != nil {
			return err
		}
		if result == "" {
			opts.Task = ""
			opts.Container = ""
			return nextExecState(ctx, ecsClient, ui.Task, opts)
		}

		opts.Container = result
		return nextExecState(ctx, ecsClient, ui.Complete, opts)
	case ui.Complete:
		return startExec(ctx, ecsClient, opts)
	}

	return errors.New("Unknown error.")
}

func startExec(ctx context.Context, ecsClient *ecs.Client, opts ExecCommandOptions) error {
	execResult, err := ecsClient.ExecuteCommand(ctx, &ecs.ExecuteCommandInput{
		Cluster:     &opts.Cluster,
		Task:        &opts.Task,
		Container:   &opts.Container,
		Interactive: opts.Interactive,
		Command:     &opts.Command,
	})
	if err != nil {
		if opts.EnableErrorChecker {
			fmt.Fprintln(os.Stderr, "Execution failed.")
			fmt.Println("Start error checking...")

			t := time.Now()
			filename := fmt.Sprintf("ecsk_%s.sh", t.Format("20060102150405"))
			err := ioutil.WriteFile(filename, checkScript, 0777)
			if err != nil {
				return err
			}

			err = util.ExecCommand("sh", "-c", fmt.Sprintf("./%s %s %s; rm -f %[1]s", filename, opts.Cluster, opts.Task))
			if err != nil {
				os.Remove(filename)
				return err
			}
			fmt.Println("Check completed.")
		}

		return err
	}

	sess, err := json.Marshal(execResult.Session)
	if err != nil {
		return err
	}

	describeResult, err := ecsClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: &opts.Cluster,
		Tasks:   []string{opts.Task},
	})
	if err != nil {
		return err
	}
	if len(describeResult.Failures) > 0 {
		return fmt.Errorf("%v", describeResult.Failures)
	}

	var runtimeId string
	for _, c := range describeResult.Tasks[0].Containers {
		if aws.StringValue(c.Name) != opts.Container {
			continue
		}
		runtimeId = *c.RuntimeId
	}
	target, err := json.Marshal(ssm.StartSessionInput{
		Target: aws.String(fmt.Sprintf("ecs:%s_%s_%s", opts.Cluster, opts.Task, runtimeId)),
	})
	if err != nil {
		return err
	}

	// Ignore SIGINT
	signalChannel := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(signalChannel, syscall.SIGINT, os.Interrupt)
	go func() {
		for {
			select {
			case <-done:
				return
			case <-signalChannel:
			}
		}
	}()
	defer close(done)

	err = util.ExecCommand(opts.Plugin, string(sess), opts.Region, "StartSession", opts.Profile, string(target), fmt.Sprintf("https://ecs.%s.amazonaws.com", opts.Region))
	if err != nil {
		return err
	}

	return nil
}
