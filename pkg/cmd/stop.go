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
	"errors"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/spf13/cobra"
	"github.com/yukiarrr/ecsk/pkg/store"
	"github.com/yukiarrr/ecsk/pkg/ui"
	"github.com/yukiarrr/ecsk/pkg/util"
)

type StopCommandOptions struct {
	Cluster string
	Tasks   []string
}

func init() {
	var opts StopCommandOptions

	stopCmd := &cobra.Command{
		Use:   "stop",
		Short: `Stop tasks like "docker stop"`,
		Long: `# ecsk stop

After selecting the task interactively, stop.`,
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

			err = nextStopState(ctx, ecsClient, ui.Cluster, opts)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		},
	}

	rootCmd.AddCommand(stopCmd)

	stopCmd.Flags().StringVar(&opts.Cluster, "cluster", "", "The short name or full Amazon Resource Name (ARN) of the cluster that hosts the task to stop. (From AWS CLI)")
	stopCmd.Flags().StringSliceVar(&opts.Tasks, "tasks", nil, "The task IDs or full Amazon Resource Name (ARN) of the tasks to stop. (From AWS CLI)")
}

func nextStopState(ctx context.Context, ecsClient *ecs.Client, state int, opts StopCommandOptions) error {
	switch state {
	case ui.Cluster:
		if opts.Cluster != "" {
			return nextStopState(ctx, ecsClient, ui.Tasks, opts)
		}

		result, err := ui.AskCluster(ctx, ecsClient, false)
		if err != nil {
			return err
		}
		if result == "" {
			return errors.New("Canceled.")
		}

		opts.Cluster = result
		return nextStopState(ctx, ecsClient, ui.Tasks, opts)
	case ui.Tasks:
		if opts.Tasks != nil {
			return nextStopState(ctx, ecsClient, ui.Complete, opts)
		}

		result, err := ui.AskTasks(ctx, ecsClient, opts.Cluster)
		if err != nil {
			return err
		}
		if result == nil {
			opts.Cluster = ""
			opts.Tasks = nil
			return nextStopState(ctx, ecsClient, ui.Cluster, opts)
		}

		opts.Tasks = result
		return nextStopState(ctx, ecsClient, ui.Complete, opts)
	case ui.Complete:
		return startStop(ctx, ecsClient, opts)
	}

	return errors.New("Unknown error.")
}

func startStop(ctx context.Context, ecsClient *ecs.Client, opts StopCommandOptions) error {
	// Insert line breaks to make the log easier to understand
	fmt.Println()

	// Immediately after stop-task, the status is still RUNNING, so wait until it changes.
	sp, err := ui.CreateSppiner(" Requesting stop-task...")
	if err != nil {
		return err
	}
	sp.Start()

	statuses := make(map[string]string)
	for _, t := range opts.Tasks {
		result, err := ecsClient.StopTask(ctx, &ecs.StopTaskInput{
			Cluster: &opts.Cluster,
			Task:    &t,
		})
		if err != nil {
			return err
		}

		statuses[t] = *result.Task.LastStatus
	}

	for {
		describeResult, err := ecsClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
			Cluster: &opts.Cluster,
			Tasks:   opts.Tasks,
		})
		if err != nil {
			return err
		}
		if len(describeResult.Failures) > 0 {
			return fmt.Errorf("%v", describeResult.Failures)
		}

		next := true
		for _, t := range describeResult.Tasks {
			if *t.LastStatus == statuses[path.Base(*t.TaskArn)] {
				next = false
				select {
				case <-ctx.Done():
					fmt.Println()
					sp.Stop()
					return nil
				case <-time.After(3 * time.Second):
				}
				break
			}
		}

		if next {
			break
		}
	}

	sp.Stop()
	fmt.Printf("%s Requested stop-task\n", ui.Green("✔︎"))

	stopProgresses := []ui.Progress{
		{Status: "DEACTIVATING", Suffix: " Deactivating...", Completed: fmt.Sprintf("%s Deactivated", ui.Green("✔︎")), PrintError: false},
		{Status: "STOPPING", Suffix: " Stopping...", Completed: fmt.Sprintf("%s Stopped", ui.Green("✔︎")), PrintError: false},
		{Status: "DEPROVISIONING", Suffix: " Deprovisioning...", Completed: fmt.Sprintf("%s Deprovisioned", ui.Green("✔︎")), PrintError: false},
	}

	for _, s := range stopProgresses {
		err := ui.PrintTaskProgress(ctx, ecsClient, opts.Cluster, opts.Tasks, s)
		if err != nil {
			return err
		}
	}

	return nil
}
