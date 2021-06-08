package launchers

import (
	"context"
	"io"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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
var envPrefix = envGetKeyPrefix()

func envGetKeyPrefix() string {
	var prefix = os.Getenv("PREFIX")
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return prefix
}

func s3listRelevantObjects() map[string]*string {
	input := s3.ListObjectsV2Input{
		Bucket: &envBucket,
		Prefix: &envPrefix,
	}
	pl := len(envPrefix)
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

func s3download(key *string) io.ReadCloser {
	input := s3.GetObjectInput{
		Bucket: &envBucket,
		Key:    key,
	}
	output, err := s3client.GetObject(ctx, &input)
	if err != nil {
		// we might want to check for 404 instead of panicking on it
		log.Panic(err)
	}
	return output.Body
}

func s3upload(key *string, body io.Reader) {
	input := s3.PutObjectInput{
		Bucket: &envBucket,
		Key:    key,
		Body:   body,
	}
	s3client.PutObject(ctx, &input)
}
