package dispatcher

import (
	"context"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
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
	if err == nil {
		s3alreadyUploadedSelf[guild.Bucket] = true
	}
	return err
}

func ec2makeServer(guild *GuildStore, script []string, variables map[string]string) error {
	return notImplemented{}
}
