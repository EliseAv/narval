package dispatcher

import (
	"context"
	"io"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func s3upload(bucket, key string, reader io.Reader) error {
	ctx := context.TODO()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}
	client := s3.NewFromConfig(cfg)
	input := s3.PutObjectInput{Bucket: &bucket, Key: &key, Body: reader}
	_, err = client.PutObject(ctx, &input)
	return err
}
