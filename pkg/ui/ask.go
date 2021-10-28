package ui

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"text/tabwriter"

	"github.com/AlecAivazis/survey/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const (
	LaunchType int = iota
	Cluster
	TaskDefinition
	Vpc
	Subnets
	SecurityGroups
	Task
	Tasks
	Container
	Bucket
	Complete
)

const Back = "← Back"
const NewBucket = "→ New Bucket"

func AskLaunchType(ecsClient *ecs.Client, addBack bool) (string, error) {
	var launchTypes []string
	if addBack {
		launchTypes = []string{Back}
	}
	launchTypes = append(launchTypes, "FARGATE", "EC2")

	prompt := &survey.Select{
		Message: "Choose Launch Type:",
		Options: launchTypes,
	}

	var launchType string
	err := survey.AskOne(prompt, &launchType)
	if err != nil {
		return "", err
	}
	if launchType == Back {
		return "", nil
	}

	return launchType, nil
}

func AskCluster(ctx context.Context, ecsClient *ecs.Client, addBack bool) (string, error) {
	result, err := ecsClient.ListClusters(ctx, &ecs.ListClustersInput{})
	if err != nil {
		return "", err
	}
	if len(result.ClusterArns) == 0 {
		return "", errors.New("No Cluster exists.")
	}

	var clusterNames []string
	if addBack {
		clusterNames = []string{Back}
	}
	for _, c := range result.ClusterArns {
		split := strings.Split(c, "/")
		clusterNames = append(clusterNames, split[len(split)-1])
	}

	prompt := &survey.Select{
		Message: "Choose Cluster:",
		Options: clusterNames,
	}

	var cluster string
	err = survey.AskOne(prompt, &cluster)
	if err != nil {
		return "", err
	}
	if cluster == Back {
		return "", nil
	}

	return cluster, nil
}

func AskTaskDefinition(ctx context.Context, ecsClient *ecs.Client, addBack bool) (string, error) {
	result, err := ecsClient.ListTaskDefinitionFamilies(ctx, &ecs.ListTaskDefinitionFamiliesInput{
		Status: "ACTIVE",
	})
	if err != nil {
		return "", err
	}
	if len(result.Families) == 0 {
		return "", errors.New("No Task Definition exists.")
	}

	var families []string
	if addBack {
		families = []string{Back}
	}
	families = append(families, result.Families...)

	prompt := &survey.Select{
		Message: "Choose Task Definition:",
		Options: families,
	}

	var taskDefinition string
	err = survey.AskOne(prompt, &taskDefinition)
	if err != nil {
		return "", err
	}
	if taskDefinition == Back {
		return "", nil
	}

	return taskDefinition, nil
}

func AskVpc(ctx context.Context, ec2Client *ec2.Client, addBack bool) (string, error) {
	result, err := ec2Client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{})
	if err != nil {
		return "", err
	}
	if len(result.Vpcs) == 0 {
		return "", errors.New("No VPC exists.")
	}

	b := strings.Builder{}
	w := tabwriter.NewWriter(&b, 0, 0, 1, ' ', tabwriter.Debug)
	var vpcIds []string
	for _, v := range result.Vpcs {
		fmt.Fprintf(w, "%s\t %s\t %s\n", *v.VpcId, *v.CidrBlock, truncate(getTagName(v.Tags)))
		vpcIds = append(vpcIds, *v.VpcId)
	}
	w.Flush()

	var opts []string
	if addBack {
		opts = []string{Back}
	}
	opts = append(opts, strings.Split(b.String(), "\n")...)
	prompt := &survey.Select{
		Message: "Choose VPC:",
		Options: opts[:len(opts)-1],
	}

	var i int
	err = survey.AskOne(prompt, &i)
	if err != nil {
		return "", err
	}
	if i == 0 {
		return "", nil
	}

	return vpcIds[i-1], err
}

func AskSubnets(ctx context.Context, ec2Client *ec2.Client, vpc string) ([]string, error) {
	result, err := ec2Client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
		Filters: []ec2types.Filter{{Name: aws.String("vpc-id"), Values: []string{vpc}}},
	})
	if err != nil {
		return nil, err
	}
	if len(result.Subnets) == 0 {
		return nil, errors.New("No Subnet exists.")
	}

	b := strings.Builder{}
	w := tabwriter.NewWriter(&b, 0, 0, 1, ' ', tabwriter.Debug)
	var subnetIds []string
	for _, s := range result.Subnets {
		fmt.Fprintf(w, "%s\t %s\t %s\n", *s.SubnetId, *s.CidrBlock, truncate(getTagName(s.Tags)))
		subnetIds = append(subnetIds, *s.SubnetId)
	}
	w.Flush()

	opts := strings.Split(b.String(), "\n")
	prompt := &survey.MultiSelect{
		Message: fmt.Sprintf("Choose Subnets %s:", Yellow("(Already filtered by VPC)")),
		Options: opts[:len(opts)-1],
	}

	var i []int
	err = survey.AskOne(prompt, &i)
	if err != nil {
		return nil, err
	}
	var subnets []string
	for _, v := range i {
		subnets = append(subnets, subnetIds[v])
	}
	if len(subnets) == 0 {
		return nil, nil
	}

	return subnets, nil
}

func AskSecurityGroups(ctx context.Context, ec2Client *ec2.Client, vpc string) ([]string, error) {
	result, err := ec2Client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		Filters: []ec2types.Filter{{Name: aws.String("vpc-id"), Values: []string{vpc}}},
	})
	if err != nil {
		return nil, err
	}
	if len(result.SecurityGroups) == 0 {
		return nil, errors.New("No Security Group exists.")
	}

	b := strings.Builder{}
	w := tabwriter.NewWriter(&b, 0, 0, 1, ' ', tabwriter.Debug)
	var securityGroupIds []string
	for _, s := range result.SecurityGroups {
		fmt.Fprintf(w, "%s\t %s\t %s\t %s\n", *s.GroupId, truncate(*s.GroupName), truncate(*s.Description), truncate(getTagName(s.Tags)))
		securityGroupIds = append(securityGroupIds, *s.GroupId)
	}
	w.Flush()

	opts := strings.Split(b.String(), "\n")
	prompt := &survey.MultiSelect{
		Message: fmt.Sprintf("Choose Security Groups %s:", Yellow("(Already filtered by VPC)")),
		Options: opts[:len(opts)-1],
	}

	var i []int
	err = survey.AskOne(prompt, &i)
	if err != nil {
		return nil, err
	}
	var securityGroups []string
	for _, v := range i {
		securityGroups = append(securityGroups, securityGroupIds[v])
	}
	if len(securityGroups) == 0 {
		return nil, nil
	}

	return securityGroups, nil
}

func AskTask(ctx context.Context, ecsClient *ecs.Client, cluster string, addBack bool) (string, error) {
	listResult, err := ecsClient.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster: &cluster,
	})
	if err != nil {
		return "", err
	}
	listSize := len(listResult.TaskArns)
	if len(listResult.TaskArns) == 0 {
		return "", errors.New("No Task exists.")
	}

	var taskArnsList [][]string
	for i := 0; i < listSize; i += 100 {
		end := i + 100
		if listSize < end {
			end = listSize
		}
		taskArnsList = append(taskArnsList, listResult.TaskArns[i:end])
	}

	var tasks []ecstypes.Task
	for _, t := range taskArnsList {
		describeResult, err := ecsClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
			Cluster: &cluster,
			Tasks:   t,
		})
		if err != nil {
			return "", err
		}
		if len(describeResult.Failures) > 0 {
			return "", fmt.Errorf("%v", describeResult.Failures)
		}
		tasks = append(tasks, describeResult.Tasks...)
	}

	b := strings.Builder{}
	w := tabwriter.NewWriter(&b, 0, 0, 1, ' ', tabwriter.Debug)
	var taskIds []string
	for _, t := range tasks {
		taskId := path.Base(*t.TaskArn)
		ipAddress := "-"
		for _, a := range t.Attachments {
			for _, d := range a.Details {
				if d.Name == nil || *d.Name != "privateIPv4Address" {
					continue
				}
				ipAddress = *d.Value
			}
		}
		fmt.Fprintf(w, "%s\t %s\t %s\t %s\t %s\t %s\n", taskId, truncate(path.Base(*t.TaskDefinitionArn)), *t.LastStatus, t.CreatedAt.Format("2006/1/2 15:04:05"), truncate(*t.Group), ipAddress)
		taskIds = append(taskIds, taskId)
	}
	w.Flush()

	var opts []string
	if addBack {
		opts = []string{Back}
	}
	opts = append(opts, strings.Split(b.String(), "\n")...)
	prompt := &survey.Select{
		Message: fmt.Sprintf("Choose Task %s:", Yellow("(Already filtered by Cluster)")),
		Options: opts[:len(opts)-1],
	}

	var i int
	err = survey.AskOne(prompt, &i)
	if err != nil {
		return "", err
	}
	if i == 0 {
		return "", nil
	}

	return taskIds[i-1], nil
}

func AskTasks(ctx context.Context, ecsClient *ecs.Client, cluster string) ([]string, error) {
	listResult, err := ecsClient.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster: &cluster,
	})
	if err != nil {
		return nil, err
	}
	listSize := len(listResult.TaskArns)
	if len(listResult.TaskArns) == 0 {
		return nil, errors.New("No Task exists.")
	}

	var taskArnsList [][]string
	for i := 0; i < listSize; i += 100 {
		end := i + 100
		if listSize < end {
			end = listSize
		}
		taskArnsList = append(taskArnsList, listResult.TaskArns[i:end])
	}

	var describeTasks []ecstypes.Task
	for _, t := range taskArnsList {
		describeResult, err := ecsClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
			Cluster: &cluster,
			Tasks:   t,
		})
		if err != nil {
			return nil, err
		}
		if len(describeResult.Failures) > 0 {
			return nil, fmt.Errorf("%v", describeResult.Failures)
		}

		describeTasks = append(describeTasks, describeResult.Tasks...)
	}

	b := strings.Builder{}
	w := tabwriter.NewWriter(&b, 0, 0, 1, ' ', tabwriter.Debug)
	var taskIds []string
	for _, t := range describeTasks {
		taskId := path.Base(*t.TaskArn)
		ipAddress := "-"
		for _, a := range t.Attachments {
			for _, d := range a.Details {
				if d.Name == nil || *d.Name != "privateIPv4Address" {
					continue
				}
				ipAddress = *d.Value
			}
		}
		fmt.Fprintf(w, "%s\t %s\t %s\t %s\t %s\t %s\n", taskId, truncate(path.Base(*t.TaskDefinitionArn)), *t.LastStatus, t.CreatedAt.Format("2006/1/2 15:04:05"), truncate(*t.Group), ipAddress)
		taskIds = append(taskIds, taskId)
	}
	w.Flush()

	opts := strings.Split(b.String(), "\n")
	prompt := &survey.MultiSelect{
		Message: fmt.Sprintf("Choose Tasks %s:", Yellow("(Already filtered by Cluster)")),
		Options: opts[:len(opts)-1],
	}

	var i []int
	err = survey.AskOne(prompt, &i)
	if err != nil {
		return nil, err
	}
	var tasks []string
	for _, v := range i {
		tasks = append(tasks, taskIds[v])
	}
	if len(tasks) == 0 {
		return nil, nil
	}

	return tasks, nil
}

func AskContainer(ctx context.Context, ecsClient *ecs.Client, cluster string, task string, addBack bool) (string, error) {
	result, err := ecsClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: &cluster,
		Tasks:   []string{task},
	})
	if err != nil {
		return "", err
	}
	if len(result.Failures) > 0 {
		return "", fmt.Errorf("%v", result.Failures)
	}
	if len(result.Tasks) == 0 {
		return "", errors.New("No Task exists.")
	}
	if len(result.Tasks[0].Containers) == 0 {
		return "", errors.New("No Container exists.")
	}

	var containerNames []string
	if addBack {
		containerNames = []string{Back}
	}
	for _, c := range result.Tasks[0].Containers {
		containerNames = append(containerNames, *c.Name)
	}

	prompt := &survey.Select{
		Message: "Choose Container:",
		Options: containerNames,
	}

	var container string
	err = survey.AskOne(prompt, &container)
	if err != nil {
		return "", err
	}
	if container == Back {
		return "", nil
	}

	return container, nil
}

func AskBucket(ctx context.Context, s3Client *s3.Client, region string, addBack bool) (string, error) {
	listResult, err := s3Client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return "", err
	}

	var bucketNames []string
	if addBack {
		bucketNames = []string{Back, NewBucket}
	}
	for _, b := range listResult.Buckets {
		bucketNames = append(bucketNames, *b.Name)
	}

	selectPrompt := &survey.Select{
		Message: fmt.Sprintf("Choose Bucket %s:", Yellow("(Use for file transfer)")),
		Options: bucketNames,
	}

	var bucket string
	err = survey.AskOne(selectPrompt, &bucket)
	if err != nil {
		return "", err
	}
	if bucket == Back {
		return "", nil
	}

	if bucket == NewBucket {
		inputPrompt := &survey.Input{
			Message: "Input Bucket Name:",
		}

		err := survey.AskOne(inputPrompt, &bucket)
		if err != nil {
			return "", err
		}

		_, err = s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
			Bucket: &bucket,
			CreateBucketConfiguration: &s3types.CreateBucketConfiguration{
				LocationConstraint: s3types.BucketLocationConstraint(region),
			},
		})
		if err != nil {
			return "", err
		}
	}

	return bucket, nil
}

func getTagName(tags []ec2types.Tag) string {
	for _, t := range tags {
		if *t.Key == "Name" {
			return *t.Value
		}
	}
	return "-"
}

func truncate(text string) string {
	const max = 30
	if len(text) > max {
		return text[:max] + "..."
	}
	return text
}
