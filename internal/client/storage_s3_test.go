package client

import (
	"encoding/json"
	"testing"
)

func TestDecodeS3UserUserQuota(t *testing.T) {
	t.Parallel()
	const raw = `{"id":7,"name":"ha001$tf","owner":"a@b.c","user_quota":{"buckets":10,"data_size_mb":512,"objects":500},"pool":{"id":1,"klass":"nvme","name":"nvme","type":"s3"}}`
	u, err := decodeS3User([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if u.Quota == nil || u.Quota.Buckets != 10 {
		t.Fatalf("quota: %+v", u.Quota)
	}
	if u.PoolID != 1 {
		t.Fatalf("pool_id: %d", u.PoolID)
	}
}

func TestPickS3PoolByName(t *testing.T) {
	t.Parallel()
	pools := []StoragePool{
		{ID: 1, Name: "nvme", Type: "s3"},
		{ID: 2, Name: "hdd-cold-21", Type: "s3"},
	}
	if p := pickS3PoolByName(pools, "NVME"); p == nil || p.ID != 1 {
		t.Fatalf("nvme: %+v", p)
	}
	if p := pickS3PoolByName(pools, " hdd-cold-21 "); p == nil || p.ID != 2 {
		t.Fatalf("hdd: %+v", p)
	}
	if pickS3PoolByName(pools, "missing") != nil {
		t.Fatal("expected nil")
	}
}

func TestEncodeBucketPathSegment(t *testing.T) {
	t.Parallel()
	if encodeBucketPathSegment("ha001/probe-bucket") != "ha001%2Fprobe-bucket" {
		t.Fatalf("got %q", encodeBucketPathSegment("ha001/probe-bucket"))
	}
}

func TestBucketJSON(t *testing.T) {
	t.Parallel()
	const raw = `{"name":"probe-bucket","path":"ha001/probe-bucket","quota":{"data_size_mb":100,"objects":50},"user_name":"ha001$tfprobe"}`
	var b S3Bucket
	if err := json.Unmarshal([]byte(raw), &b); err != nil {
		t.Fatal(err)
	}
	if b.Path != "ha001/probe-bucket" {
		t.Fatalf("path %q", b.Path)
	}
}
