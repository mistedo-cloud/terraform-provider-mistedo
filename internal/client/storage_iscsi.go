package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// IscsiConfig is an iSCSI target configuration (one per pool tier in typical deployments).
type IscsiConfig struct {
	ID        int          `json:"id"`
	Name      string       `json:"name"`
	TargetIQN string       `json:"target_iqn"`
	Pool      *StoragePool `json:"pool,omitempty"`
}

// IscsiDisk is a block volume in an iSCSI config.
type IscsiDisk struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Owner  string `json:"owner"`
	SizeGB int    `json:"size_gb"`
}

// IscsiDiskCreate is the body for POST /iscsi/configs/{id}/disks.
type IscsiDiskCreate struct {
	Name   string `json:"name"`
	Owner  string `json:"owner"`
	SizeGB int    `json:"size_gb"`
}

// IscsiClientDiskRef identifies a disk when assigning to an iSCSI client (POST body item).
type IscsiClientDiskRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// IscsiClient is an iSCSI initiator (consumer) record.
type IscsiClient struct {
	ID           int                  `json:"id,omitempty"`
	Name         string               `json:"name"`
	Owner        string               `json:"owner"`
	IQN          string               `json:"iqn"`
	ChapUsername string               `json:"chap_username,omitempty"`
	ChapPassword string               `json:"chap_password,omitempty"`
	AccountName  string               `json:"account_name,omitempty"`
	Disks        []IscsiClientDiskRef `json:"disks,omitempty"`
}

// IscsiClientPutBody is the JSON body for PUT /iscsi/clients/{id}. The API rejects unknown fields (e.g. id in body).
type IscsiClientPutBody struct {
	Name         string `json:"name"`
	Owner        string `json:"owner"`
	IQN          string `json:"iqn"`
	ChapUsername string `json:"chap_username,omitempty"`
	ChapPassword string `json:"chap_password,omitempty"`
}

// ListIscsiConfigs returns GET /iscsi/configs.
func (c *Client) ListIscsiConfigs(ctx context.Context) ([]IscsiConfig, error) {
	path := StorageAPIPrefix + "/iscsi/configs"
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
	var out []IscsiConfig
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode iSCSI configs: %w", err)
	}
	return out, nil
}

// GetIscsiConfig returns GET /iscsi/configs/{config_id}.
func (c *Client) GetIscsiConfig(ctx context.Context, configID int) (*IscsiConfig, error) {
	path := StorageAPIPrefix + "/iscsi/configs/" + strconv.Itoa(configID)
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
	var out IscsiConfig
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode iSCSI config: %w", err)
	}
	return &out, nil
}

// FindIscsiConfigByPoolName matches a config whose pool name equals poolName (case-insensitive).
func (c *Client) FindIscsiConfigByPoolName(ctx context.Context, poolName string) (*IscsiConfig, error) {
	want := strings.ToLower(strings.TrimSpace(poolName))
	if want == "" {
		return nil, fmt.Errorf("%w: empty pool name", ErrNotFound)
	}
	configs, err := c.ListIscsiConfigs(ctx)
	if err != nil {
		return nil, err
	}
	for i := range configs {
		if configs[i].Pool != nil && strings.EqualFold(strings.TrimSpace(configs[i].Pool.Name), poolName) {
			return &configs[i], nil
		}
	}
	return nil, fmt.Errorf("%w: iSCSI config for pool %q not found", ErrNotFound, strings.TrimSpace(poolName))
}

// ListIscsiDisksInConfig returns GET /iscsi/configs/{config_id}/disks.
func (c *Client) ListIscsiDisksInConfig(ctx context.Context, configID int) ([]IscsiDisk, error) {
	path := StorageAPIPrefix + "/iscsi/configs/" + strconv.Itoa(configID) + "/disks"
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
	var out []IscsiDisk
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode iSCSI disks: %w", err)
	}
	return out, nil
}

// GetIscsiDiskInConfig finds a disk by id within a config.
func (c *Client) GetIscsiDiskInConfig(ctx context.Context, configID int, diskID int) (*IscsiDisk, error) {
	disks, err := c.ListIscsiDisksInConfig(ctx, configID)
	if err != nil {
		return nil, err
	}
	for i := range disks {
		if disks[i].ID == diskID {
			return &disks[i], nil
		}
	}
	return nil, fmt.Errorf("%w: iSCSI disk %d in config %d", ErrNotFound, diskID, configID)
}

// FindIscsiDiskConfig finds which config contains diskID by scanning configs (for import / recovery).
func (c *Client) FindIscsiDiskConfig(ctx context.Context, diskID int) (configID int, disk *IscsiDisk, err error) {
	configs, err := c.ListIscsiConfigs(ctx)
	if err != nil {
		return 0, nil, err
	}
	for i := range configs {
		disks, err := c.ListIscsiDisksInConfig(ctx, configs[i].ID)
		if err != nil {
			return 0, nil, err
		}
		for j := range disks {
			if disks[j].ID == diskID {
				return configs[i].ID, &disks[j], nil
			}
		}
	}
	return 0, nil, fmt.Errorf("%w: iSCSI disk id %d", ErrNotFound, diskID)
}

// CreateIscsiDisk calls POST /iscsi/configs/{config_id}/disks.
func (c *Client) CreateIscsiDisk(ctx context.Context, configID int, in IscsiDiskCreate) (*IscsiDisk, error) {
	body, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	path := StorageAPIPrefix + "/iscsi/configs/" + strconv.Itoa(configID) + "/disks"
	resp, err := c.Do(ctx, http.MethodPost, path, body)
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
	var out IscsiDisk
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode iSCSI disk: %w", err)
	}
	return &out, nil
}

// UpdateIscsiDisk calls PUT /iscsi/configs/{config_id}/disks/{disk_id}.
func (c *Client) UpdateIscsiDisk(ctx context.Context, configID int, disk *IscsiDisk) (*IscsiDisk, error) {
	body, err := json.Marshal(disk)
	if err != nil {
		return nil, err
	}
	path := StorageAPIPrefix + "/iscsi/configs/" + strconv.Itoa(configID) + "/disks/" + strconv.Itoa(disk.ID)
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
	var out IscsiDisk
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode iSCSI disk: %w", err)
	}
	return &out, nil
}

// DeleteIscsiDisk calls DELETE /iscsi/configs/{config_id}/disks/{disk_id}.
func (c *Client) DeleteIscsiDisk(ctx context.Context, configID int, diskID int) error {
	path := StorageAPIPrefix + "/iscsi/configs/" + strconv.Itoa(configID) + "/disks/" + strconv.Itoa(diskID)
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

// ListIscsiClients returns GET /iscsi/clients.
func (c *Client) ListIscsiClients(ctx context.Context) ([]IscsiClient, error) {
	path := StorageAPIPrefix + "/iscsi/clients"
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
	var out []IscsiClient
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode iSCSI clients: %w", err)
	}
	return out, nil
}

// GetIscsiClient returns a client by id (list + filter; no single-GET in API).
func (c *Client) GetIscsiClient(ctx context.Context, clientID int) (*IscsiClient, error) {
	clients, err := c.ListIscsiClients(ctx)
	if err != nil {
		return nil, err
	}
	for i := range clients {
		if clients[i].ID == clientID {
			return &clients[i], nil
		}
	}
	return nil, fmt.Errorf("%w: iSCSI client id %d", ErrNotFound, clientID)
}

// CreateIscsiClient calls POST /iscsi/clients.
func (c *Client) CreateIscsiClient(ctx context.Context, in *IscsiClient) (*IscsiClient, error) {
	body, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	path := StorageAPIPrefix + "/iscsi/clients"
	resp, err := c.Do(ctx, http.MethodPost, path, body)
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
	var out IscsiClient
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode iSCSI client: %w", err)
	}
	return &out, nil
}

// AssignIscsiClientDisks calls POST /iscsi/clients/{client_id}/disks with a JSON array of {id,name}.
func (c *Client) AssignIscsiClientDisks(ctx context.Context, clientID int, assignments []IscsiClientDiskRef) error {
	if len(assignments) == 0 {
		return nil
	}
	body, err := json.Marshal(assignments)
	if err != nil {
		return err
	}
	path := StorageAPIPrefix + "/iscsi/clients/" + strconv.Itoa(clientID) + "/disks"
	resp, err := c.Do(ctx, http.MethodPost, path, body)
	if err != nil {
		return err
	}
	b, err := readResponseBody(resp)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return NewAPIError(resp, b)
	}
	return nil
}

// UnassignIscsiClientDisk calls DELETE /iscsi/clients/{client_id}/disks/{disk_id}.
func (c *Client) UnassignIscsiClientDisk(ctx context.Context, clientID int, diskID int) error {
	path := StorageAPIPrefix + "/iscsi/clients/" + strconv.Itoa(clientID) + "/disks/" + strconv.Itoa(diskID)
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

// UpdateIscsiClient calls PUT /iscsi/clients/{client_id}. Disk assignments are not sent here; use Assign/Unassign.
func (c *Client) UpdateIscsiClient(ctx context.Context, in *IscsiClient) (*IscsiClient, error) {
	body, err := json.Marshal(IscsiClientPutBody{
		Name:         in.Name,
		Owner:        in.Owner,
		IQN:          in.IQN,
		ChapUsername: in.ChapUsername,
		ChapPassword: in.ChapPassword,
	})
	if err != nil {
		return nil, err
	}
	path := StorageAPIPrefix + "/iscsi/clients/" + strconv.Itoa(in.ID)
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
	var out IscsiClient
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode iSCSI client: %w", err)
	}
	return &out, nil
}

// DeleteIscsiClient calls DELETE /iscsi/clients/{client_id}.
func (c *Client) DeleteIscsiClient(ctx context.Context, clientID int) error {
	path := StorageAPIPrefix + "/iscsi/clients/" + strconv.Itoa(clientID)
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
