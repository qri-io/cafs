package gcloud

import (
	"context"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/qri-io/cafs/test"
	"google.golang.org/api/option"
)

func TestCache(t *testing.T) {
	ctx := context.Background()
	cli, err := storage.NewClient(ctx, option.WithoutAuthentication())
	if err != nil {
		t.Errorf("error creating client: %s", err.Error())
		return
	}

	cf := NewCacheFunc(ctx, cli, func(c *CacheCfg) {
		c.Full = true
		c.BucketName = "qri_tests"
	})

	test.RunCacheTests(cf, t)
}
