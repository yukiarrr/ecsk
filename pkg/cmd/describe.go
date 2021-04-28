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
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/TylerBrock/colorjson"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/spf13/cobra"
	"github.com/yukiarrr/ecsk/pkg/store"
	"github.com/yukiarrr/ecsk/pkg/ui"
	"github.com/yukiarrr/ecsk/pkg/util"
)

type DescribeCommandOptions struct {
	Cluster string
	Tasks   []string
}

func init() {
	var opts DescribeCommandOptions

	execCmd := &cobra.Command{
		Use:   "describe",
		Short: `View detailed information like "docker inspect"`,
		Long: `# ecsk describe

After selecting the tasks interactively, view detailed information.
You can also use it to check a task list.`,
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

			err = nextDescribeState(ctx, ecsClient, ui.Cluster, opts)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		},
	}

	rootCmd.AddCommand(execCmd)

	execCmd.Flags().StringVar(&opts.Cluster, "cluster", "", "The short name or full Amazon Resource Name (ARN) of the cluster that hosts the task.")
	execCmd.Flags().StringSliceVar(&opts.Tasks, "tasks", nil, "The task IDs or full Amazon Resource Name (ARN) of the tasks.")
}

func nextDescribeState(ctx context.Context, ecsClient *ecs.Client, state int, opts DescribeCommandOptions) error {
	switch state {
	case ui.Cluster:
		if opts.Cluster != "" {
			return nextDescribeState(ctx, ecsClient, ui.Tasks, opts)
		}

		result, err := ui.AskCluster(ctx, ecsClient, false)
		if err != nil {
			return err
		}
		if result == "" {
			return errors.New("Canceled.")
		}

		opts.Cluster = result
		return nextDescribeState(ctx, ecsClient, ui.Tasks, opts)
	case ui.Tasks:
		if opts.Tasks != nil {
			return nextDescribeState(ctx, ecsClient, ui.Complete, opts)
		}

		result, err := ui.AskTasks(ctx, ecsClient, opts.Cluster)
		if err != nil {
			return err
		}
		if result == nil {
			opts.Cluster = ""
			opts.Tasks = nil
			return nextDescribeState(ctx, ecsClient, ui.Cluster, opts)
		}

		opts.Tasks = result
		return nextDescribeState(ctx, ecsClient, ui.Complete, opts)
	case ui.Complete:
		return startDescribe(ctx, ecsClient, opts)
	}

	return errors.New("Unknown error.")
}

func startDescribe(ctx context.Context, ecsClient *ecs.Client, opts DescribeCommandOptions) error {
	result, err := ecsClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: &opts.Cluster,
		Tasks:   opts.Tasks,
	})
	if err != nil {
		return err
	}
	if len(result.Failures) > 0 {
		return fmt.Errorf("%v", result.Failures)
	}

	t, err := json.Marshal(result)
	if err != nil {
		return err
	}

	var obj map[string]interface{}
	json.Unmarshal(t, &obj)

	f := colorjson.NewFormatter()
	f.Indent = 2
	s, err := f.Marshal(obj)
	if err != nil {
		return err
	}

	fmt.Println(string(s))

	return nil
}
