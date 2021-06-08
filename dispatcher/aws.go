package dispatcher

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var s3alreadyUploadedSelf = map[string]bool{}

func s3upload(guild *GuildStore, key string, reader io.Reader) error {
	ctx := context.TODO()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}
	cfg.Region = guild.Region
	client := s3.NewFromConfig(cfg)
	input := s3.PutObjectInput{Bucket: &guild.Bucket, Key: &key, Body: reader}
	_, err = client.PutObject(ctx, &input)
	return err
}

func s3uploadSelf(guild *GuildStore) error {
	if s3alreadyUploadedSelf[guild.Bucket] {
		return nil
	}
	var err error
	path, err := os.Executable()
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	err = s3upload(guild, "narval", file)
	if err != nil {
		return err
	}
	s3alreadyUploadedSelf[guild.Bucket] = true
	return nil
}

func ec2makeServer(guild *GuildStore, variables map[string]string) error {
	ctx := context.TODO()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}
	cfg.Region = guild.Region
	client := ec2.NewFromConfig(cfg)

	// launch the instance :D
	imageId, err := ec2getBestAmi(ctx, client)
	if err != nil {
		return err
	}
	input := ec2.RunInstancesInput{
		ImageId:      imageId,
		InstanceType: types.InstanceTypeC5aLarge,
		UserData:     variablesToLauncherScript(variables),
	}
	_, err = client.RunInstances(ctx, &input)
	return err
}

func ec2getBestAmi(ctx context.Context, client *ec2.Client) (*string, error) {
	amiInput := ec2.DescribeImagesInput{Filters: []types.Filter{{
		Name:   aws.String("name"),
		Values: []string{"amzn2-ami-hvm-*-x86_64-gp2"},
	}}}
	amiOutput, err := client.DescribeImages(ctx, &amiInput)
	if err != nil {
		return nil, err
	}
	mostRecentIndex := 0
	mostRecentDate := ""
	for index, image := range amiOutput.Images {
		if mostRecentDate < *image.CreationDate {
			mostRecentIndex = index
			mostRecentDate = *image.CreationDate
		}
	}
	return amiOutput.Images[mostRecentIndex].Name, nil
}

func variablesToLauncherScript(variables map[string]string) *string {
	var builder strings.Builder
	for name, value := range variables {
		value = strings.ReplaceAll(value, "'", "'\\''")
		builder.WriteString(fmt.Sprintf("export %s='%s'\n", name, value))
	}
	builder.WriteString("aws s3 cp s3://$BUCKET/narval /narval\n")
	builder.WriteString("chmod +x /narval\n")
	builder.WriteString("/narval\n")
	return aws.String(builder.String())
}
