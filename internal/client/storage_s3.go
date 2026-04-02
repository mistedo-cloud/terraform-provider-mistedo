package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// StorageAPIPrefix is the Mistedo Storage HTTP API (Ceph RGW control plane), v2 only.
const StorageAPIPrefix = "/api/storage/v2"

// StoragePool is a storage pool from GET /pools?type=<s3|iscsi|...>.
type StoragePool struct {
	ID    int    `json:"id"`
	Class string `json:"klass"`
	Name  string `json:"name"`
	Type  string `json:"type"`
}

// S3UserQuota maps to API quota / user_quota (Ceph s3user limits).
type S3UserQuota struct {
	Buckets    int `json:"buckets"`
	DataSizeMB int `json:"data_size_mb"`
	Objects    int `json:"objects"`
}

// S3UserUsage is read-only usage from the API.
type S3UserUsage struct {
	Buckets    int `json:"buckets"`
	DataSizeMB int `json:"data_size_mb"`
	Objects    int `json:"objects"`
}

// KeysS3 holds S3 protocol credentials (Ceph RGW).
type KeysS3 struct {
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	User      string `json:"user"`
	Active    bool   `json:"active,omitempty"`
}

// KeysSwift holds Swift protocol credentials.
type KeysSwift struct {
	SecretKey string `json:"secret_key"`
	User      string `json:"user"`
	Active    bool   `json:"active,omitempty"`
}

// S3UserKeys groups protocol keys returned by the API.
type S3UserKeys struct {
	S3    *KeysS3    `json:"s3,omitempty"`
	Swift *KeysSwift `json:"swift,omitempty"`
}

// StorageAccountRef is a minimal account object embedded in responses.
type StorageAccountRef struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// S3User is a technical Ceph RGW user (Storage API: /s3/users).
type S3User struct {
	ID          int                `json:"id"`
	Description string             `json:"description,omitempty"`
	Name        string             `json:"name"`
	Owner       string             `json:"owner,omitempty"`
	PoolID      int                `json:"pool_id,omitempty"`
	Pool        *StoragePool       `json:"pool,omitempty"`
	Account     *StorageAccountRef `json:"account,omitempty"`
	Quota       *S3UserQuota       `json:"quota,omitempty"`
	UserQuota   *S3UserQuota       `json:"user_quota,omitempty"`
	Keys        *S3UserKeys        `json:"keys,omitempty"`
	Usage       *S3UserUsage       `json:"usage,omitempty"`
	Status      string             `json:"status,omitempty"`
	IsLocked    bool               `json:"is_locked,omitempty"`
}

// S3UserCreate is the JSON body for POST /s3/users.
type S3UserCreate struct {
	PoolID      int          `json:"pool_id"`
	Owner       string       `json:"owner"`
	Description string       `json:"description,omitempty"`
	Name        string       `json:"name,omitempty"`
	Quota       *S3UserQuota `json:"quota,omitempty"`
}

// BucketQuota is per-bucket quota (data + objects).
type BucketQuota struct {
	DataSizeMB int `json:"data_size_mb"`
	Objects    int `json:"objects"`
}

// BucketUsage is read-only usage for a bucket.
type BucketUsage struct {
	DataSizeMB       int `json:"data_size_mb"`
	Objects          int `json:"objects"`
	TotalObjects     int `json:"total_objects,omitempty"`
	MultipartObjects int `json:"multipart_objects,omitempty"`
}

// S3Bucket is an object storage bucket (Ceph bucket).
type S3Bucket struct {
	Name     string       `json:"name"`
	Path     string       `json:"path,omitempty"`
	UserName string       `json:"user_name"`
	Quota    *BucketQuota `json:"quota,omitempty"`
	Usage    *BucketUsage `json:"usage,omitempty"`
}

// FindS3PoolByName resolves pool id from the human-readable pool name (GET /pools?type=s3).
// Matching is case-insensitive and trims whitespace.
func (c *Client) FindS3PoolByName(ctx context.Context, name string) (*StoragePool, error) {
	pools, err := c.ListS3Pools(ctx)
	if err != nil {
		return nil, err
	}
	p := pickS3PoolByName(pools, name)
	if p == nil {
		return nil, fmt.Errorf("%w: S3 pool named %q not found (see GET /api/storage/v2/pools?type=s3)", ErrNotFound, strings.TrimSpace(name))
	}
	return p, nil
}

// FindIscsiPoolByName resolves pool id from the pool name (GET /pools?type=iscsi).
func (c *Client) FindIscsiPoolByName(ctx context.Context, name string) (*StoragePool, error) {
	pools, err := c.ListIscsiPools(ctx)
	if err != nil {
		return nil, err
	}
	p := pickS3PoolByName(pools, name)
	if p == nil {
		return nil, fmt.Errorf("%w: iSCSI pool named %q not found (see GET /api/storage/v2/pools?type=iscsi)", ErrNotFound, strings.TrimSpace(name))
	}
	return p, nil
}

func pickS3PoolByName(pools []StoragePool, name string) *StoragePool {
	want := strings.ToLower(strings.TrimSpace(name))
	if want == "" {
		return nil
	}
	for i := range pools {
		if strings.ToLower(strings.TrimSpace(pools[i].Name)) == want {
			return &pools[i]
		}
	}
	return nil
}

// ListS3Pools returns storage pools where type=s3.
func (c *Client) ListS3Pools(ctx context.Context) ([]StoragePool, error) {
	return c.listStoragePools(ctx, "s3")
}

// ListIscsiPools returns storage pools where type=iscsi (block / iSCSI tier).
func (c *Client) ListIscsiPools(ctx context.Context) ([]StoragePool, error) {
	return c.listStoragePools(ctx, "iscsi")
}

func (c *Client) listStoragePools(ctx context.Context, poolType string) ([]StoragePool, error) {
	path := StorageAPIPrefix + "/pools?type=" + url.QueryEscape(poolType)
	resp, err := c.Do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	b, err := readResponseBody(resp)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, NewAPIError(resp, b)
	}
	var out []StoragePool
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode pools: %w", err)
	}
	return out, nil
}

// CreateS3User creates a Ceph S3 user (POST /s3/users).
func (c *Client) CreateS3User(ctx context.Context, in S3UserCreate) (*S3User, error) {
	body, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(ctx, http.MethodPost, StorageAPIPrefix+"/s3/users", body)
	if err != nil {
		return nil, err
	}
	b, err := readResponseBody(resp)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, NewAPIError(resp, b)
	}
	return decodeS3User(b)
}

// GetS3User returns GET /s3/users/{id}.
func (c *Client) GetS3User(ctx context.Context, id int) (*S3User, error) {
	path := fmt.Sprintf("%s/s3/users/%d", StorageAPIPrefix, id)
	resp, err := c.Do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	b, err := readResponseBody(resp)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, NewAPIError(resp, b)
	}
	return decodeS3User(b)
}

// UpdateS3User sends PUT /s3/users/{id} with a full S3 user object (read-modify-write in the provider).
func (c *Client) UpdateS3User(ctx context.Context, id int, u *S3User) (*S3User, error) {
	body, err := json.Marshal(u)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("%s/s3/users/%d", StorageAPIPrefix, id)
	resp, err := c.Do(ctx, http.MethodPut, path, body)
	if err != nil {
		return nil, err
	}
	b, err := readResponseBody(resp)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, NewAPIError(resp, b)
	}
	return decodeS3User(b)
}

// DeleteS3User deletes DELETE /s3/users/{id}.
func (c *Client) DeleteS3User(ctx context.Context, id int) error {
	path := fmt.Sprintf("%s/s3/users/%d", StorageAPIPrefix, id)
	resp, err := c.Do(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	b, err := readResponseBody(resp)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return NewAPIError(resp, b)
	}
	return nil
}

// CreateS3Bucket POST /s3/buckets.
func (c *Client) CreateS3Bucket(ctx context.Context, bkt *S3Bucket) (*S3Bucket, error) {
	body, err := json.Marshal(bkt)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(ctx, http.MethodPost, StorageAPIPrefix+"/s3/buckets", body)
	if err != nil {
		return nil, err
	}
	rb, err := readResponseBody(resp)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, NewAPIError(resp, rb)
	}
	var out S3Bucket
	if err := json.Unmarshal(rb, &out); err != nil {
		return nil, fmt.Errorf("decode bucket: %w", err)
	}
	return &out, nil
}

// encodeBucketPathSegment encodes the bucket path for use in /s3/buckets/{path}.
func encodeBucketPathSegment(bucketPath string) string {
	return url.PathEscape(bucketPath)
}

// UpdateS3Bucket PUT /s3/buckets/{path}.
func (c *Client) UpdateS3Bucket(ctx context.Context, bucketPath string, bkt *S3Bucket) (*S3Bucket, error) {
	body, err := json.Marshal(bkt)
	if err != nil {
		return nil, err
	}
	p := fmt.Sprintf("%s/s3/buckets/%s", StorageAPIPrefix, encodeBucketPathSegment(bucketPath))
	resp, err := c.Do(ctx, http.MethodPut, p, body)
	if err != nil {
		return nil, err
	}
	rb, err := readResponseBody(resp)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, NewAPIError(resp, rb)
	}
	var out S3Bucket
	if err := json.Unmarshal(rb, &out); err != nil {
		return nil, fmt.Errorf("decode bucket: %w", err)
	}
	return &out, nil
}

// DeleteS3Bucket DELETE /s3/buckets/{path}.
func (c *Client) DeleteS3Bucket(ctx context.Context, bucketPath string) error {
	p := fmt.Sprintf("%s/s3/buckets/%s", StorageAPIPrefix, encodeBucketPathSegment(bucketPath))
	resp, err := c.Do(ctx, http.MethodDelete, p, nil)
	if err != nil {
		return err
	}
	b, err := readResponseBody(resp)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return NewAPIError(resp, b)
	}
	return nil
}

// ListS3Buckets lists buckets, optionally filtered by S3 user name (filter[user.name]).
func (c *Client) ListS3Buckets(ctx context.Context, filterUserName string) ([]S3Bucket, error) {
	u := StorageAPIPrefix + "/s3/buckets"
	if strings.TrimSpace(filterUserName) != "" {
		q := url.Values{}
		q.Set("filter[user.name]", filterUserName)
		u += "?" + q.Encode()
	}
	resp, err := c.Do(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	b, err := readResponseBody(resp)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, NewAPIError(resp, b)
	}
	var out []S3Bucket
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode buckets: %w", err)
	}
	return out, nil
}

// GetS3BucketByPath finds a bucket by its full path (e.g. ha001/my-bucket).
// If userName is set, lists with filter[user.name] first; if not found, lists all buckets (for import by path only).
func (c *Client) GetS3BucketByPath(ctx context.Context, bucketPath string, userName string) (*S3Bucket, error) {
	try := func(filter string) (*S3Bucket, error) {
		list, err := c.ListS3Buckets(ctx, filter)
		if err != nil {
			return nil, err
		}
		for i := range list {
			if list[i].Path == bucketPath {
				return &list[i], nil
			}
		}
		return nil, nil
	}
	if strings.TrimSpace(userName) != "" {
		if b, err := try(userName); err != nil || b != nil {
			if err != nil {
				return nil, err
			}
			return b, nil
		}
	}
	if b, err := try(""); err != nil || b != nil {
		if err != nil {
			return nil, err
		}
		return b, nil
	}
	return nil, fmt.Errorf("%w: bucket path %q", ErrNotFound, bucketPath)
}

func decodeS3User(b []byte) (*S3User, error) {
	var raw struct {
		S3User
		UserQuota *S3UserQuota `json:"user_quota"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	out := raw.S3User
	if out.Quota == nil && raw.UserQuota != nil {
		out.Quota = raw.UserQuota
	}
	if out.Pool != nil && out.PoolID == 0 {
		out.PoolID = out.Pool.ID
	}
	return &out, nil
}

// ParseS3UserID parses import id string to int.
func ParseS3UserID(s string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(s))
}

// ErrInvalidImportID indicates a malformed resource import id.
var ErrInvalidImportID = errors.New("invalid import id")
