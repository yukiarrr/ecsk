package store

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
)

func NewConfig(ctx context.Context, region string, profile string, code string) (aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithSharedConfigProfile(profile),
		config.WithAssumeRoleCredentialOptions(func(options *stscreds.AssumeRoleOptions) {
			options.TokenProvider = func() (string, error) {
				if code != "" {
					return code, nil
				}
				return stscreds.StdinTokenProvider()
			}
		}),
	)
	if err != nil {
		return aws.Config{}, err
	}
	return cfg, err
}
