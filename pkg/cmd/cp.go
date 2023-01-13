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
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/spf13/cobra"
	"github.com/yukiarrr/ecsk/pkg/store"
	"github.com/yukiarrr/ecsk/pkg/ui"
	"github.com/yukiarrr/ecsk/pkg/util"
)

type CpCommandOptions struct {
	Cluster    string
	Task       string
	Container  string
	Bucket     string
	Src        string
	Dst        string
	FromLocal  bool
	FromRemote bool
	Plugin     string
	Command    string
	Region     string
	Profile    string
	Arm64      bool
}

const Format = "sh -c 'type curl > /dev/null 2>&1 && curl -s %s -o %s || wget -q -O %[2]s %[1]s; chmod +x ./%[2]s && ./%[2]s %d %s %s %s && rm -f ./%[2]s'"

var DownloadUrl = fmt.Sprintf("https://raw.githubusercontent.com/yukiarrr/ecsk/%s/bin/cp", Version)

func init() {
	var opts CpCommandOptions

	cpCmd := &cobra.Command{
		Use:   "cp",
		Short: `Transfer files like "docker cp"`,
		Long: `# ecsk cp ./ [container_name]:/etc/nginx/

After selecting the task interactively, copy the files from local to remote.
Internally, using an S3 Bucket to transfer the files, so you need to add permissions for the corresponding Bucket to the task role.

If you want to select the container interactively, use "ecsk cp . / :/etc/nginx/".


# ecsk cp [container_name]:/var/log/nginx/access.log ./

Transfer files from remote to local.`,
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
			s3Client := s3.NewFromConfig(cfg)
			opts.Region = cfg.Region
			opts.Profile = profile

			if len(args) != 2 {
				fmt.Fprintln(os.Stderr, `Wrong format. Try "ecsk cp --help".`)
				os.Exit(1)
			}

			src := strings.Split(args[0], ":")
			dst := strings.Split(args[1], ":")
			if len(src) > 1 {
				opts.FromRemote = true
				if src[0] != "" {
					opts.Container = src[0]
				}
				if !filepath.IsAbs(src[1]) {
					fmt.Fprintln(os.Stderr, `The remote path must be absolute. Try "ecsk cp --help".`)
					os.Exit(1)
				}
				opts.Src = src[1]
				opts.Dst = dst[0]
			} else if len(dst) > 1 {
				opts.FromLocal = true
				if dst[0] != "" {
					opts.Container = dst[0]
				}
				if !filepath.IsAbs(dst[1]) {
					fmt.Fprintln(os.Stderr, `The remote path must be absolute. Try "ecsk cp --help".`)
					os.Exit(1)
				}
				opts.Src = src[0]
				opts.Dst = dst[1]
			} else {
				fmt.Fprintln(os.Stderr, `Wrong format. Try "ecsk cp --help".`)
				os.Exit(1)
			}

			err = nextCpState(ctx, ecsClient, s3Client, ui.Cluster, opts)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		},
	}

	rootCmd.AddCommand(cpCmd)

	cpCmd.Flags().StringVar(&opts.Cluster, "cluster", "", "The short name or full Amazon Resource Name (ARN) of the cluster that hosts the task.")
	cpCmd.Flags().StringVar(&opts.Task, "task", "", "The task ID or full Amazon Resource Name (ARN) of the task.")
	cpCmd.Flags().StringVar(&opts.Container, "container", "", "The name of the container to copy files.")
	cpCmd.Flags().StringVar(&opts.Bucket, "bucket", "", "The bucket to use for file transfer.")
	cpCmd.Flags().StringVar(&opts.Plugin, "plugin", "session-manager-plugin", "Path of session-manager-plugin.")
	cpCmd.Flags().BoolVar(&opts.Arm64, "arm64", false, "Container CPU is ARM64.")
}

func nextCpState(ctx context.Context, ecsClient *ecs.Client, s3Client *s3.Client, state int, opts CpCommandOptions) error {
	switch state {
	case ui.Cluster:
		if opts.Cluster != "" {
			return nextCpState(ctx, ecsClient, s3Client, ui.Task, opts)
		}

		result, err := ui.AskCluster(ctx, ecsClient, false)
		if err != nil {
			return err
		}
		if result == "" {
			return errors.New("Canceled.")
		}

		opts.Cluster = result
		return nextCpState(ctx, ecsClient, s3Client, ui.Task, opts)
	case ui.Task:
		if opts.Task != "" {
			return nextCpState(ctx, ecsClient, s3Client, ui.Container, opts)
		}

		result, err := ui.AskTask(ctx, ecsClient, opts.Cluster, true)
		if err != nil {
			return err
		}
		if result == "" {
			opts.Cluster = ""
			opts.Task = ""
			return nextCpState(ctx, ecsClient, s3Client, ui.Cluster, opts)
		}

		opts.Task = result
		return nextCpState(ctx, ecsClient, s3Client, ui.Container, opts)
	case ui.Container:
		if opts.Container != "" {
			return nextCpState(ctx, ecsClient, s3Client, ui.Bucket, opts)
		}

		result, err := ui.AskContainer(ctx, ecsClient, opts.Cluster, opts.Task, true)
		if err != nil {
			return err
		}
		if result == "" {
			opts.Task = ""
			opts.Container = ""
			return nextCpState(ctx, ecsClient, s3Client, ui.Task, opts)
		}

		opts.Container = result
		return nextCpState(ctx, ecsClient, s3Client, ui.Bucket, opts)
	case ui.Bucket:
		if opts.Bucket != "" {
			return nextCpState(ctx, ecsClient, s3Client, ui.Complete, opts)
		}

		result, err := ui.AskBucket(ctx, s3Client, opts.Region, true)
		if err != nil {
			return err
		}
		if result == "" {
			opts.Container = ""
			opts.Bucket = ""
			return nextCpState(ctx, ecsClient, s3Client, ui.Task, opts)
		}

		opts.Bucket = result
		return nextCpState(ctx, ecsClient, s3Client, ui.Complete, opts)
	case ui.Complete:
		return startCp(ctx, ecsClient, s3Client, opts)
	}

	return errors.New("Unknown error.")
}

func startCp(ctx context.Context, ecsClient *ecs.Client, s3Client *s3.Client, opts CpCommandOptions) error {
	t := time.Now()
	key := "ecsk_" + t.Format("20060102150405")

	var keys []string

	if opts.FromLocal {
		var err error
		keys, err = store.Upload(ctx, s3Client, opts.Bucket, key, opts.Src)
		if err != nil {
			return err
		}

		err = startExec(ctx, ecsClient, ExecCommandOptions{
			Cluster:            opts.Cluster,
			Task:               opts.Task,
			Container:          opts.Container,
			Interactive:        true,
			Plugin:             opts.Plugin,
			EnableErrorChecker: false,
			Command:            fmt.Sprintf(Format, DownloadUrl, key, 1, opts.Bucket, key, filepath.ToSlash(opts.Dst)),
			Region:             opts.Region,
			Profile:            opts.Profile,
		})
		if err != nil {
			return err
		}
	} else if opts.FromRemote {
		err := startExec(ctx, ecsClient, ExecCommandOptions{
			Cluster:            opts.Cluster,
			Task:               opts.Task,
			Container:          opts.Container,
			Interactive:        true,
			Plugin:             opts.Plugin,
			EnableErrorChecker: false,
			Command:            fmt.Sprintf(Format, DownloadUrl, key, 0, opts.Bucket, key, filepath.ToSlash(opts.Src)),
			Region:             opts.Region,
			Profile:            opts.Profile,
		})
		if err != nil {
			return err
		}

		keys, err = store.Download(ctx, s3Client, opts.Bucket, key, opts.Dst)
		if err != nil {
			return err
		}
	} else {
		return errors.New("Unknown error.")
	}

	var objects []types.ObjectIdentifier
	for _, k := range keys {
		clone := k
		// Safety
		if !strings.Contains(clone, key) {
			continue
		}
		objects = append(objects, types.ObjectIdentifier{
			Key: &clone,
		})
	}

	if len(objects) > 0 {
		_, err := s3Client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: &opts.Bucket,
			Delete: &types.Delete{
				Objects: objects,
				Quiet:   false,
			},
		})
		if err != nil {
			return err
		}
	}

	return nil
}
