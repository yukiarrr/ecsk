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
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/spf13/cobra"
	"github.com/yukiarrr/ecsk/pkg/store"
	"github.com/yukiarrr/ecsk/pkg/ui"
	"github.com/yukiarrr/ecsk/pkg/util"
)

type RunCommandOptions struct {
	LaunchType           string
	Cluster              string
	TaskDefinition       string
	Vpc                  string
	Subnets              []string
	SecurityGroups       []string
	AssignPublicIp       bool
	EnableExecuteCommand bool
	Count                int32
	Overrides            string
	Rm                   bool
	Detach               bool
	Container            string
	Interactive          bool
	Command              string
	Plugin               string
	Region               string
	Profile              string
	Code                 string
}

func init() {
	var opts RunCommandOptions

	runCmd := &cobra.Command{
		Use:   "run",
		Short: `Run tasks like "docker run"`,
		Long: `# ecsk run

If you don't specify any flags, after entering task information interactively, the log will continue to flow until the task is started and stopped as in "docker run".


# ecsk run -e -i --rm -c [container_name] -- [command]

After the task is started, execute the command specified by execute-command.
By specifying --rm, the task will be automatically stopped at the end of the session, so you can operate it like a bastion host.`,
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
			ec2Client := ec2.NewFromConfig(cfg)
			opts.Region = cfg.Region
			opts.Profile = profile
			opts.Code = code

			argsLenAtDash := cmd.ArgsLenAtDash()
			if argsLenAtDash > -1 {
				opts.Command = strings.Join(args[argsLenAtDash:], " ")
				if opts.Container == "" {
					fmt.Fprintln(os.Stderr, "Need -c or --container to exec command.")
					os.Exit(1)
					return
				}

			}

			askedOpts, taskIds, err := nextRunState(ctx, ecsClient, ec2Client, ui.LaunchType, opts)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}

			if taskIds == nil || !opts.Rm || opts.Detach {
				return
			}

			ctx, cancel = context.WithCancel(context.Background())
			util.HandleSignals(cancel)

			err = startStop(ctx, ecsClient, StopCommandOptions{
				Cluster: askedOpts.Cluster,
				Tasks:   taskIds,
			})
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		},
	}

	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringVar(&opts.LaunchType, "launch-type", "", "The launch type on which to run your task. The accepted values are FARGATE and EC2. (From AWS CLI)")
	runCmd.Flags().StringVar(&opts.Cluster, "cluster", "", "The short name or full Amazon Resource Name (ARN) of the cluster on which to run your task. (From AWS CLI)")
	runCmd.Flags().StringVar(&opts.TaskDefinition, "task-definition", "", "The family and revision (family:revision ) or full ARN of the task definition to run. If a revision is not specified, the latest ACTIVE revision is used. (From AWS CLI)")
	runCmd.Flags().StringVar(&opts.Vpc, "vpc", "", "Filtering subnets and security groups.")
	runCmd.Flags().StringSliceVar(&opts.Subnets, "subnets", nil, "The IDs of the subnets associated with the task or service. (From AWS CLI)")
	runCmd.Flags().StringSliceVar(&opts.SecurityGroups, "security-groups", nil, "The IDs of the security groups associated with the task or service. (From AWS CLI)")
	runCmd.Flags().BoolVar(&opts.AssignPublicIp, "assign-public-ip", false, "Whether the task's elastic network interface receives a public IP address. (From AWS CLI)")
	runCmd.Flags().BoolVarP(&opts.EnableExecuteCommand, "enable-execute-command", "e", false, "Whether or not to enable the execute command functionality for the containers in this task. If true , this enables execute command functionality on all containers in the task. (From AWS CLI)")
	runCmd.Flags().Int32Var(&opts.Count, "count", 1, "The number of instantiations of the specified task to place on your cluster. You can specify up to 10 tasks per call.	(From AWS CLI)")
	runCmd.Flags().StringVar(&opts.Overrides, "overrides", "{}", "A list of container overrides in JSON format that specify the name of a container in the specified task definition and the overrides it should receive. You can override the default command for a container (that is specified in the task definition or Docker image) with a command override. You can also override existing environment variables (that are specified in the task definition or Docker image) on a container or add new environment variables to it with an environment override. (From AWS CLI)")
	runCmd.Flags().BoolVar(&opts.Rm, "rm", false, "When CLI is stoped, tasks are also stoped.")
	runCmd.Flags().BoolVarP(&opts.Detach, "detach", "d", false, "Do not wait for tasks to start and stop.")
	runCmd.Flags().StringVarP(&opts.Container, "container", "c", "", "The name of the container to execute the command on. (From AWS CLI)")
	runCmd.Flags().BoolVarP(&opts.Interactive, "interactive", "i", false, "Use this flag to run your command in interactive mode. (From AWS CLI)")
	runCmd.Flags().StringVar(&opts.Plugin, "plugin", "session-manager-plugin", "Path of the session-manager-plugin.")
}

func nextRunState(ctx context.Context, ecsClient *ecs.Client, ec2Client *ec2.Client, state int, opts RunCommandOptions) (RunCommandOptions, []string, error) {
	switch state {
	case ui.LaunchType:
		if opts.LaunchType != "" {
			return nextRunState(ctx, ecsClient, ec2Client, ui.TaskDefinition, opts)
		}

		result, err := ui.AskLaunchType(ecsClient, false)
		if err != nil {
			return RunCommandOptions{}, nil, err
		}
		if result == "" {
			return RunCommandOptions{}, nil, errors.New("Canceled.")
		}

		opts.LaunchType = result
		return nextRunState(ctx, ecsClient, ec2Client, ui.Cluster, opts)
	case ui.Cluster:
		if opts.Cluster != "" {
			return nextRunState(ctx, ecsClient, ec2Client, ui.TaskDefinition, opts)
		}

		result, err := ui.AskCluster(ctx, ecsClient, true)
		if err != nil {
			return RunCommandOptions{}, nil, err
		}
		if result == "" {
			opts.LaunchType = ""
			opts.Cluster = ""
			return nextRunState(ctx, ecsClient, ec2Client, ui.LaunchType, opts)
		}

		opts.Cluster = result
		return nextRunState(ctx, ecsClient, ec2Client, ui.Cluster, opts)
	case ui.TaskDefinition:
		if opts.TaskDefinition != "" {
			return nextRunState(ctx, ecsClient, ec2Client, ui.Vpc, opts)
		}

		result, err := ui.AskTaskDefinition(ctx, ecsClient, true)
		if err != nil {
			return RunCommandOptions{}, nil, err
		}
		if result == "" {
			opts.Cluster = ""
			opts.TaskDefinition = ""
			return nextRunState(ctx, ecsClient, ec2Client, ui.Cluster, opts)
		}

		opts.TaskDefinition = result
		return nextRunState(ctx, ecsClient, ec2Client, ui.Vpc, opts)
	case ui.Vpc:
		if opts.Vpc != "" || (opts.Subnets != nil && opts.SecurityGroups != nil) {
			return nextRunState(ctx, ecsClient, ec2Client, ui.Subnets, opts)
		}

		result, err := ui.AskVpc(ctx, ec2Client, true)
		if err != nil {
			return RunCommandOptions{}, nil, err
		}
		if result == "" {
			opts.TaskDefinition = ""
			opts.Vpc = ""
			return nextRunState(ctx, ecsClient, ec2Client, ui.TaskDefinition, opts)
		}

		opts.Vpc = result
		return nextRunState(ctx, ecsClient, ec2Client, ui.Subnets, opts)
	case ui.Subnets:
		if opts.Subnets != nil {
			return nextRunState(ctx, ecsClient, ec2Client, ui.SecurityGroups, opts)
		}

		result, err := ui.AskSubnets(ctx, ec2Client, opts.Vpc)
		if err != nil {
			return RunCommandOptions{}, nil, err
		}
		if result == nil {
			opts.Vpc = ""
			opts.Subnets = nil
			return nextRunState(ctx, ecsClient, ec2Client, ui.Vpc, opts)
		}

		opts.Subnets = result
		return nextRunState(ctx, ecsClient, ec2Client, ui.SecurityGroups, opts)
	case ui.SecurityGroups:
		if opts.SecurityGroups != nil {
			return nextRunState(ctx, ecsClient, ec2Client, ui.Complete, opts)
		}

		result, err := ui.AskSecurityGroups(ctx, ec2Client, opts.Vpc)
		if err != nil {
			return RunCommandOptions{}, nil, err
		}
		if result == nil {
			opts.Subnets = nil
			opts.SecurityGroups = nil
			return nextRunState(ctx, ecsClient, ec2Client, ui.Subnets, opts)
		}

		opts.SecurityGroups = result
		return nextRunState(ctx, ecsClient, ec2Client, ui.Complete, opts)
	case ui.Complete:
		taskIds, err := startRun(ctx, ecsClient, opts)
		return opts, taskIds, err
	}

	return RunCommandOptions{}, nil, errors.New("Unknown error.")
}

func startRun(ctx context.Context, ecsClient *ecs.Client, opts RunCommandOptions) ([]string, error) {
	var assignPublicIp types.AssignPublicIp
	if opts.AssignPublicIp {
		assignPublicIp = "ENABLED"
	} else {
		assignPublicIp = "DISABLED"
	}

	var launchType types.LaunchType
	if opts.LaunchType == "FARGATE" {
		launchType = "FARGATE"
	} else {
		launchType = "EC2"
	}

	var overrides types.TaskOverride
	err := json.Unmarshal([]byte(opts.Overrides), &overrides)
	if err != nil {
		return nil, err
	}

	runResult, err := ecsClient.RunTask(ctx, &ecs.RunTaskInput{
		LaunchType:     launchType,
		Cluster:        &opts.Cluster,
		TaskDefinition: &opts.TaskDefinition,
		Count:          &opts.Count,
		NetworkConfiguration: &types.NetworkConfiguration{
			AwsvpcConfiguration: &types.AwsVpcConfiguration{
				Subnets:        opts.Subnets,
				SecurityGroups: opts.SecurityGroups,
				AssignPublicIp: assignPublicIp,
			},
		},
		Overrides:            &overrides,
		EnableExecuteCommand: opts.EnableExecuteCommand,
	})
	if err != nil {
		return nil, err
	}
	if len(runResult.Failures) > 0 {
		return nil, fmt.Errorf("%v", runResult.Failures)
	}

	var taskIds []string
	for _, t := range runResult.Tasks {
		taskIds = append(taskIds, path.Base(*t.TaskArn))
	}

	if opts.Detach {
		return taskIds, nil
	}

	done := make(chan bool, 1)
	isExecuting := false
	executed := make(chan bool, 1)

	go func() {
		startProgresses := []ui.Progress{
			{Status: "PROVISIONING", Suffix: " Provisioning...", Completed: fmt.Sprintf("%s Provisioned", ui.Green("✔︎")), PrintError: true},
			{Status: "PENDING", Suffix: " Pending...", Completed: fmt.Sprintf("%s Pended", ui.Green("✔︎")), PrintError: true},
			{Status: "ACTIVATING", Suffix: " Activating...", Completed: fmt.Sprintf("%s Activated", ui.Green("✔︎")), PrintError: true},
		}

		for _, s := range startProgresses {
			err = ui.PrintTaskProgress(ctx, ecsClient, opts.Cluster, taskIds, s)
			if err != nil {
				// Insert line breaks to make the log easier to understand
				fmt.Println()
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
				return
			}

			select {
			case <-ctx.Done():
				return
			default:
			}
		}

		fmt.Printf("Tasks started! %s\n", taskIds)

		if opts.Command == "" {
			go func() {
				err := startLogs(ctx, ecsClient, LogsCommandOptions{
					Cluster: opts.Cluster,
					Tasks:   taskIds,
					Since:   "5m",
					Region:  opts.Region,
					Profile: opts.Profile,
					Code:    opts.Code,
				})
				if err != nil {
					fmt.Println("Wait until tasks stopped...")
				}
			}()

			_ = waitUntilTasksStopped(ctx, ecsClient, opts.Cluster, taskIds)
		} else {
			fmt.Println("Waiting for the execute command agent...")

			defer close(executed)

			const max = 10
			for i := 0; i < max; i++ {
				isExecuting = true

				err := startExec(ctx, ecsClient, ExecCommandOptions{
					Cluster:            opts.Cluster,
					Task:               taskIds[0],
					Container:          opts.Container,
					Interactive:        opts.Interactive,
					Plugin:             opts.Plugin,
					EnableErrorChecker: false,
					Command:            opts.Command,
					Region:             opts.Region,
				},
				)

				isExecuting = false

				if err != nil {
					if i < max-1 && strings.Contains(err.Error(), "the execute command agent isn’t running") {
						select {
						case <-ctx.Done():
							return
						case <-time.After(3 * time.Second):
						}
						continue
					}

					fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
					return
				}

				break
			}
		}

		close(done)
	}()

	select {
	case <-ctx.Done():
	case <-done:
	}

	if isExecuting {
		<-executed
	}

	return taskIds, nil
}

func waitUntilTasksStopped(ctx context.Context, ecsClient *ecs.Client, cluster string, taskIds []string) error {
	return ecs.NewTasksStoppedWaiter(ecsClient).Wait(ctx, &ecs.DescribeTasksInput{
		Cluster: &cluster,
		Tasks:   taskIds,
	}, 30*time.Minute)
}
