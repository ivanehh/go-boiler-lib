package azure

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
)

type AzSharedKeyCreds struct {
	Account string `yaml:"account" json:"account"`
	Key     string `yaml:"key" json:"key"`
	Url     string `yaml:"url" json:"url"`
}

type AzureContainerClient struct {
	c         *azblob.Client
	creds     AzSharedKeyCreds
	container string
}

type AzureClientConfig struct {
	Container   string           `yaml:"container" json:"container"`
	Credentials AzSharedKeyCreds `yaml:"credentials" json:"credentials"`
}

// NewAzContainerClient creates a new container client with the provided configuration; the client is immutable
func NewAzContainerClient(config AzureClientConfig) (*AzureContainerClient, error) {
	client := new(AzureContainerClient)
	client.creds = config.Credentials
	client.container = config.Container

	cred, err := azblob.NewSharedKeyCredential(client.creds.Account, client.creds.Key)
	if err != nil {
		return nil, err
	}
	client.c, err = azblob.NewClientWithSharedKeyCredential(client.creds.Url, cred, nil)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (acc *AzureContainerClient) sanitizeName(n string) string {
	s := strings.Split(n, ".")
	return strings.Join([]string{s[0], s[len(s)-1]}, ".")
}

func (acc *AzureContainerClient) UploadBuffer(ctx context.Context, blob string, content bytes.Buffer) error {
	_, err := acc.c.UploadBuffer(ctx, acc.container, blob, content.Bytes(), nil)
	if err != nil {
		return err
	}
	return nil
}

func (acc *AzureContainerClient) UploadFile(ctx context.Context, content *os.File, blobdir string) error {
	fname := filepath.Base(content.Name())
	blob := acc.sanitizeName(fname)
	_, err := acc.c.UploadFile(ctx, acc.container, path.Join(blobdir, blob), content, nil)
	if err != nil {
		return err
	}
	return err
}

func (acc *AzureContainerClient) Enumerate(ctx context.Context) ([]string, error) {
	items := make([]string, 0)
	pager := acc.c.NewListBlobsFlatPager(acc.container, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, item := range page.ListBlobsFlatSegmentResponse.Segment.BlobItems {
			items = append(items, *item.Name)
		}
	}
	return items, nil
}

var ErrDestinationTooSmall = errors.New("the provided destination can not fit the content of the blob")

func (acc *AzureContainerClient) PullBuffer(ctx context.Context, item string, destination *[]byte) error {
	_, err := acc.c.DownloadBuffer(
		ctx,
		acc.container,
		item,
		*destination,
		nil,
	)
	if err != nil {
		return err
	}
	return nil
}

func (acc *AzureContainerClient) PullFile(ctx context.Context, item string, destination *os.File) error {
	_, err := acc.c.DownloadFile(
		ctx,
		acc.container,
		item,
		destination,
		nil,
	)
	if err != nil {
		return err
	}
	return nil
}

func (acc *AzureContainerClient) DeleteBlob(ctx context.Context, item string) error {
	_, err := acc.c.DeleteBlob(ctx, acc.container, item, nil)
	return err
}
