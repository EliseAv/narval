package launchers

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

var ctx = context.Background()
var cfg = awsLoadConfig()
var s3client = s3.NewFromConfig(cfg)

func awsLoadConfig() aws.Config {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Panic(err)
	}
	return cfg
}

var envBucket = os.Getenv("BUCKET")
var envPrefix = ensureItsAFolder(os.Getenv("PREFIX"))

func ensureItsAFolder(path string) string {
	if path != "" && !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return path
}

func s3listRelevantObjects(prefix string) map[string]*string {
	input := s3.ListObjectsV2Input{
		Bucket: &envBucket,
		Prefix: aws.String(ensureItsAFolder(envPrefix + prefix)),
	}
	pl := len(*input.Prefix)
	result := map[string]*string{}
	for {
		output, err := s3client.ListObjectsV2(ctx, &input)
		if err != nil {
			log.Panic(err)
		}
		for _, value := range output.Contents {
			result[(*value.Key)[pl:]] = value.Key
		}
		if output.NextContinuationToken == nil {
			return result
		}
		input.ContinuationToken = output.NextContinuationToken
	}
}

func s3download(name string) io.ReadCloser {
	input := s3.GetObjectInput{
		Bucket: &envBucket,
		Key:    aws.String(envPrefix + name),
	}
	output, err := s3client.GetObject(ctx, &input)
	if err != nil {
		var noSuchKey *types.NoSuchKey
		if errors.As(err, &noSuchKey) {
			return nil
		} else {
			log.Panic(err)
		}
	}
	return output.Body
}

func s3upload(name string, body io.Reader) error {
	input := s3.PutObjectInput{
		Bucket: &envBucket,
		Key:    aws.String(envPrefix + name),
		Body:   body,
	}
	_, err := s3client.PutObject(ctx, &input)
	return err
}
