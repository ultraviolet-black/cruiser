package s3

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	s3manager "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ultraviolet-black/cruiser/pkg/state"
)

type tfstateObject struct {
	tfstate *state.Tfstate
	etag    string
}

type tfstateSource struct {
	s3Client *s3.Client

	bucket string

	tfstateObjects map[string]*tfstateObject
}

func (t *tfstateSource) GetTfstate(ctx context.Context) ([]*state.Tfstate, error) {

	tfstates := []*state.Tfstate{}

	paginator := s3.NewListObjectsV2Paginator(t.s3Client, &s3.ListObjectsV2Input{
		Bucket: aws.String(t.bucket),
	})

	downloader := s3manager.NewDownloader(t.s3Client)

	existingObjects := make(map[string]struct{})

	needUpdate := false

	for paginator.HasMorePages() {

		object, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		objectName := aws.ToString(object.Name)

		if !strings.HasSuffix(objectName, ".tfstate") {
			continue
		}

		head, err := t.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(t.bucket),
			Key:    object.Name,
		})
		if err != nil {
			return nil, err
		}

		etag := aws.ToString(head.ETag)

		objectFullName := fmt.Sprintf("%s/%s", t.bucket, objectName)

		tfstateObj, ok := t.tfstateObjects[objectFullName]
		if !ok {
			tfstateObj = &tfstateObject{}
		}

		if tfstateObj.etag != etag {
			buf := make([]byte, head.ContentLength)

			writeAtBuf := s3manager.NewWriteAtBuffer(buf)

			_, err := downloader.Download(ctx, writeAtBuf, &s3.GetObjectInput{
				Bucket: aws.String(t.bucket),
				Key:    object.Name,
			})
			if err != nil {
				return nil, err
			}

			tfstate := &state.Tfstate{}

			if err := json.Unmarshal(writeAtBuf.Bytes(), tfstate); err != nil {
				return nil, err
			}

			tfstateObj.tfstate = tfstate

			needUpdate = true
		}

		tfstateObj.etag = etag

		tfstates = append(tfstates, tfstateObj.tfstate)

		existingObjects[objectFullName] = struct{}{}

		t.tfstateObjects[objectFullName] = tfstateObj

	}

	if !needUpdate {
		return nil, nil
	}

	toDelete := []string{}

	for objectFullName := range t.tfstateObjects {
		if _, ok := existingObjects[objectFullName]; ok {
			continue
		}

		toDelete = append(toDelete, objectFullName)
	}

	for _, objectFullName := range toDelete {
		delete(t.tfstateObjects, objectFullName)
	}

	return tfstates, nil

}
