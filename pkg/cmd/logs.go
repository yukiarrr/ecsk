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
	"regexp"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/knqyf263/utern/cloudwatch"
	"github.com/knqyf263/utern/config"
	"github.com/spf13/cobra"
	"github.com/yukiarrr/ecsk/pkg/store"
	"github.com/yukiarrr/ecsk/pkg/ui"
	"github.com/yukiarrr/ecsk/pkg/util"
)

type LogsCommandOptions struct {
	Cluster string
	Tasks   []string
	Since   string
	Region  string
	Profile string
	Code    string
}

func init() {
	var opts LogsCommandOptions

	logsCmd := &cobra.Command{
		Use:   "logs",
		Short: `View logs like "docker logs"`,
		Long: `# ecsk logs

After selecting the task interactively, view logs.
Multiple tasks can be specified.`,
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
			opts.Code = code

			err = nextLogsState(ctx, ecsClient, ui.Cluster, opts)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		},
	}

	rootCmd.AddCommand(logsCmd)

	logsCmd.Flags().StringVar(&opts.Cluster, "cluster", "", "The short name or full Amazon Resource Name (ARN) of the cluster that hosts the task.")
	logsCmd.Flags().StringSliceVar(&opts.Tasks, "tasks", nil, "The task IDs or full Amazon Resource Name (ARN) of the tasks.")
	logsCmd.Flags().StringVar(&opts.Since, "since", "5m", "Return logs newer than a relative duration like 52, 2m, or 3h. (From utern)")
}

func nextLogsState(ctx context.Context, ecsClient *ecs.Client, state int, opts LogsCommandOptions) error {
	switch state {
	case ui.Cluster:
		if opts.Cluster != "" {
			return nextLogsState(ctx, ecsClient, ui.Tasks, opts)
		}

		result, err := ui.AskCluster(ctx, ecsClient, false)
		if err != nil {
			return err
		}
		if result == "" {
			return errors.New("Canceled.")
		}

		opts.Cluster = result
		return nextLogsState(ctx, ecsClient, ui.Tasks, opts)
	case ui.Tasks:
		if opts.Tasks != nil {
			return nextLogsState(ctx, ecsClient, ui.Complete, opts)
		}

		result, err := ui.AskTasks(ctx, ecsClient, opts.Cluster)
		if err != nil {
			return err
		}
		if result == nil {
			opts.Cluster = ""
			opts.Tasks = nil
			return nextLogsState(ctx, ecsClient, ui.Cluster, opts)
		}

		opts.Tasks = result
		return nextLogsState(ctx, ecsClient, ui.Complete, opts)
	case ui.Complete:
		return startLogs(ctx, ecsClient, opts)
	}

	return errors.New("Unknown error.")
}

func startLogs(ctx context.Context, ecsClient *ecs.Client, opts LogsCommandOptions) error {
	now := time.Now()
	startDuration, err := time.ParseDuration(opts.Since)
	var startTime time.Time
	if err != nil {
		startTime, err = time.Parse(time.RFC3339, opts.Since)
		if err != nil {
			return err
		}
	} else {
		startTime = now.Add(-startDuration)
	}

	describeTaskResult, err := ecsClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: &opts.Cluster,
		Tasks:   opts.Tasks,
	})
	if err != nil {
		return err
	}
	if len(describeTaskResult.Failures) > 0 {
		return fmt.Errorf("%v", describeTaskResult.Failures)
	}

	var groupFilter string
	var streamFilter string

	for i, t := range describeTaskResult.Tasks {
		describeTaskDefinitionResult, err := ecsClient.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
			TaskDefinition: t.TaskDefinitionArn,
		})
		if err != nil {
			return err
		}

		for _, c := range describeTaskDefinitionResult.TaskDefinition.ContainerDefinitions {
			if c.LogConfiguration == nil {
				continue
			}

			g := regexp.QuoteMeta(c.LogConfiguration.Options["awslogs-group"])
			if g == "" {
				continue
			}

			if groupFilter != "" {
				groupFilter = groupFilter + "|"
			}
			groupFilter = groupFilter + g
		}

		streamFilter = streamFilter + path.Base(*t.TaskArn)
		if i < len(describeTaskResult.Tasks)-1 {
			streamFilter = streamFilter + "|"
		}
	}

	if groupFilter == "" {
		return errors.New("Log Group not found.")
	}

	client := cloudwatch.NewClient(&config.Config{
		StartTime:           &startTime,
		EndTime:             nil,
		LogGroupNameFilter:  regexp.MustCompile(groupFilter),
		LogStreamNameFilter: regexp.MustCompile(streamFilter),
		LogStreamNamePrefix: "",
		FilterPattern:       "",
		Profile:             opts.Profile,
		Code:                opts.Code,
		Region:              opts.Region,
		Timestamps:          true,
		EventID:             false,
		NoLogGroupName:      true,
		NoLogStreamName:     false,
		MaxLength:           0,
		Color:               true,
	})
	_ = client.Tail(ctx)

	return nil
}
