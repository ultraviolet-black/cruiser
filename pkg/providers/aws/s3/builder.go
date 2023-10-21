package s3

import (
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ultraviolet-black/cruiser/pkg/state"
)

type TfstateSourceOption func(*tfstateSource)

func WithTfstateBucket(bucketName string) TfstateSourceOption {
	return func(t *tfstateSource) {
		t.bucket = bucketName
	}
}

func WithS3ClientFactory(s3ClientFactory func() *s3.Client) TfstateSourceOption {
	return func(t *tfstateSource) {
		t.s3Client = s3ClientFactory
	}
}

func NewTfstateSource(opts ...TfstateSourceOption) state.TfstateSource {

	t := &tfstateSource{
		tfstateObjects: make(map[string]*tfstateObject),
	}

	for _, opt := range opts {
		opt(t)
	}

	return t

}
