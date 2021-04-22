package ui

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/briandowns/spinner"
)

type Progress struct {
	Status     string
	Suffix     string
	Completed  string
	PrintError bool
}

func CreateSppiner(suffix string) (*spinner.Spinner, error) {
	sp := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	sp.Suffix = suffix
	err := sp.Color("fgYellow")
	return sp, err
}

func PrintTaskProgress(ctx context.Context, ecsClient *ecs.Client, cluster string, taskIds []string, p Progress) error {
	sp, err := CreateSppiner(p.Suffix)
	if err != nil {
		return err
	}
	sp.Start()

	var describeResult *ecs.DescribeTasksOutput
	for {
		describeResult, err = ecsClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
			Cluster: &cluster,
			Tasks:   taskIds,
		})
		if err != nil {
			return err
		}
		if len(describeResult.Failures) > 0 {
			return fmt.Errorf("%v", describeResult.Failures)
		}

		next := true
		for _, t := range describeResult.Tasks {
			if *t.LastStatus == p.Status {
				select {
				case <-ctx.Done():
					fmt.Println()
					sp.Stop()
					return nil
				case <-time.After(3 * time.Second):
				}
				next = false
				break
			}
		}
		if next {
			break
		}
	}

	for _, t := range describeResult.Tasks {
		if p.PrintError && t.StoppedReason != nil {
			return errors.New(*t.StoppedReason)
		}
	}

	sp.Stop()
	fmt.Println(p.Completed)

	return nil
}
