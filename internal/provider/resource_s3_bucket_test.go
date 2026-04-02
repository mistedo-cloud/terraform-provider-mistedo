package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/mistedo/terraform-provider-mistedo/internal/client"
)

func TestQuotaIntFromAPIWithPrior(t *testing.T) {
	t.Parallel()
	if !quotaIntFromAPIWithPrior(-1, types.Int64Null()).IsNull() {
		t.Fatal("expected null for -1 when prior omitted")
	}
	if v := quotaIntFromAPIWithPrior(-1, types.Int64Value(-1)); v.IsNull() || v.ValueInt64() != -1 {
		t.Fatalf("explicit -1: %+v", v)
	}
	if v := quotaIntFromAPIWithPrior(0, types.Int64Null()); v.IsNull() || v.ValueInt64() != 0 {
		t.Fatalf("0: %+v", v)
	}
	if v := quotaIntFromAPIWithPrior(100, types.Int64Null()); v.ValueInt64() != 100 {
		t.Fatalf("100: %+v", v)
	}
}

func TestS3BucketModelFromAPI_quotaSentinels(t *testing.T) {
	t.Parallel()
	m := s3BucketModelFromAPI(&client.S3Bucket{
		Name:     "b",
		Path:     "ha001/b",
		UserName: "ha001$u",
		Quota:    &client.BucketQuota{DataSizeMB: -1, Objects: -1},
	}, s3BucketModel{})
	if !m.QuotaDataSizeMB.IsNull() || !m.QuotaObjects.IsNull() {
		t.Fatalf("expected null quotas, got data_size=%v objects=%v", m.QuotaDataSizeMB, m.QuotaObjects)
	}
}

func TestS3BucketModelFromAPI_mixedQuota(t *testing.T) {
	t.Parallel()
	m := s3BucketModelFromAPI(&client.S3Bucket{
		Name:     "b",
		Path:     "ha001/b",
		UserName: "ha001$u",
		Quota:    &client.BucketQuota{DataSizeMB: 256, Objects: -1},
	}, s3BucketModel{})
	if m.QuotaDataSizeMB.ValueInt64() != 256 {
		t.Fatal(m.QuotaDataSizeMB)
	}
	if !m.QuotaObjects.IsNull() {
		t.Fatal("objects should be null when -1 and prior omitted")
	}
}

func TestS3BucketModelFromAPI_explicitUnlimited(t *testing.T) {
	t.Parallel()
	prior := s3BucketModel{
		QuotaDataSizeMB: types.Int64Value(-1),
		QuotaObjects:    types.Int64Value(-1),
	}
	m := s3BucketModelFromAPI(&client.S3Bucket{
		Name:     "b",
		Path:     "ha001/b",
		UserName: "ha001$u",
		Quota:    &client.BucketQuota{DataSizeMB: -1, Objects: -1},
	}, prior)
	if m.QuotaDataSizeMB.ValueInt64() != -1 || m.QuotaObjects.ValueInt64() != -1 {
		t.Fatalf("expected -1 in state when set in config: %+v %+v", m.QuotaDataSizeMB, m.QuotaObjects)
	}
}

func TestS3BucketModelFromAPI_noQuotaObject(t *testing.T) {
	t.Parallel()
	m := s3BucketModelFromAPI(&client.S3Bucket{
		Name:     "b",
		Path:     "ha001/b",
		UserName: "ha001$u",
		Quota:    nil,
	}, s3BucketModel{})
	if !m.QuotaDataSizeMB.IsNull() || !m.QuotaObjects.IsNull() {
		t.Fatal("expected null when Quota nil")
	}
}

func TestS3UserModelFromAPI_quotaSentinels(t *testing.T) {
	t.Parallel()
	m := s3UserModelFromAPI(&client.S3User{
		ID:   1,
		Name: "ha001$u",
		Quota: &client.S3UserQuota{
			Buckets:    -1,
			DataSizeMB: -1,
			Objects:    -1,
		},
	}, s3UserModel{Name: types.StringValue("u"), PoolName: types.StringValue("nvme")})
	if !m.QuotaBuckets.IsNull() || !m.QuotaDataSizeMB.IsNull() || !m.QuotaObjects.IsNull() {
		t.Fatalf("expected null quotas: %+v %+v %+v", m.QuotaBuckets, m.QuotaDataSizeMB, m.QuotaObjects)
	}
}
