package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/mistedo/terraform-provider-mistedo/internal/client"
)

var _ resource.Resource = (*instanceGroupResource)(nil)

type instanceGroupResource struct {
	client *client.Client
}

func newInstanceGroupResource() resource.Resource {
	return &instanceGroupResource{}
}

func (r *instanceGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_instance_group"
}

type instanceGroupModel struct {
	ID                   types.String `tfsdk:"id"`
	Name                 types.String `tfsdk:"name"`
	Description          types.String `tfsdk:"description"`
	TemplateID           types.String `tfsdk:"template_id"`
	DesiredInstanceCount types.Int64  `tfsdk:"desired_instance_count"`
	CPUCores             types.Int64  `tfsdk:"cpu_cores"`
	MemoryMiB            types.Int64  `tfsdk:"memory_mib"`
	BootDisk             types.Object `tfsdk:"boot_disk"`
	DataDisk             types.Object `tfsdk:"data_disk"`
	Subnet               types.String `tfsdk:"subnet"`
	PrivateIPv4Cidr      types.String `tfsdk:"private_ipv4_cidr"`
	PrivateIPAllocation  types.String `tfsdk:"private_ip_allocation"`
	PrivateIPv4Address   types.String `tfsdk:"private_ipv4_address"`
	SecurityGroupIds     types.List   `tfsdk:"security_group_ids"`
	PassAuth             types.String `tfsdk:"pass_auth"`
	Password             types.String `tfsdk:"password"`
	SshPublicKeys        types.List   `tfsdk:"ssh_public_keys"`
	PublicRemoteAccess   types.List   `tfsdk:"public_remote_access"`
	ManagedAccess        types.String `tfsdk:"managed_access"`
	UserData             types.String `tfsdk:"user_data"`

	LifecycleState types.String `tfsdk:"lifecycle_state"`
	RegionNumber   types.String `tfsdk:"region_number"`
	Vlan           types.String `tfsdk:"vlan"`
	Instances      types.List   `tfsdk:"instances"`
}

type igDiskSpecModel struct {
	SizeGib types.Int64  `tfsdk:"size_gib"`
	Type    types.String `tfsdk:"type"`
}

func (r *instanceGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	diskAttrs := map[string]schema.Attribute{
		"size_gib": schema.Int64Attribute{
			Required:            true,
			MarkdownDescription: "Disk size in **gibibytes (GiB)**.",
		},
		"type": schema.StringAttribute{
			Required:            true,
			MarkdownDescription: "Storage type (e.g. `nvme`).",
		},
	}
	instBootAttrs := map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Disk id in the compute API.",
		},
		"size_gib": schema.Int64Attribute{
			Computed:            true,
			MarkdownDescription: "Size in gibibytes (from API bytes).",
		},
		"storage_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Backing storage id from the API.",
		},
		"filename": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Disk filename / uuid used by reconfigure APIs.",
		},
	}
	instDataAttrs := map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Disk id in the compute API.",
		},
		"size_gib": schema.Int64Attribute{
			Computed:            true,
			MarkdownDescription: "Size in gibibytes.",
		},
		"storage_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Backing storage id from the API.",
		},
		"filename": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Disk filename / uuid.",
		},
	}
	nicAttrs := map[string]schema.Attribute{
		"subnet": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Network name the allocation belongs to.",
		},
		"nic": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "NIC name when set.",
		},
		"ip": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "IPv4 address for `type = nic` when assigned.",
		},
		"hostname": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Hostname from the allocation when set.",
		},
		"mac": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "MAC address when set.",
		},
		"type": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Allocation type from the API (e.g. `nic`).",
		},
	}
	instAttrs := map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "VM id.",
		},
		"name": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "VM name.",
		},
		"boot_disk": schema.SingleNestedAttribute{
			Computed:            true,
			MarkdownDescription: "Boot disk (first disk after sorting by id).",
			Attributes:          instBootAttrs,
		},
		"data_disks": schema.ListNestedAttribute{
			Computed:            true,
			MarkdownDescription: "All non-boot disks (includes disks added after create).",
			NestedObject: schema.NestedAttributeObject{
				Attributes: instDataAttrs,
			},
		},
		"network_interfaces": schema.ListNestedAttribute{
			Computed:            true,
			MarkdownDescription: "Network allocations for this VM from the service `networks` payload.",
			NestedObject: schema.NestedAttributeObject{
				Attributes: nicAttrs,
			},
		},
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "A **service** (instance group) ordered from the compute catalog: several VMs from one template. " +
			"Create uses `POST /api/compute/v1/service_orders/cart/service_requests`. " +
			"`private_ipv4_cidr` is always taken from the named **subnet** in cloud networks (never set manually).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ManageIQ **service** id.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Service name (`service_name` in the order).",
				Validators:          []validator.String{stringvalidator.LengthBetween(1, 256)},
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Service description.",
			},
			"template_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Catalog template id (from `mistedo_instance_template`).",
			},
			"desired_instance_count": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Target number of VMs (`number_of_vms`).",
				Validators:          []validator.Int64{int64validator.Between(1, 500)},
			},
			"cpu_cores": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "vCPU count per VM (`cpu`).",
			},
			"memory_mib": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "RAM per VM in **mebibytes** (`vm_memory`).",
			},
			"boot_disk": schema.SingleNestedAttribute{
				Required:            true,
				MarkdownDescription: "Boot / system disk for each VM.",
				Attributes:          diskAttrs,
			},
			"data_disk": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "At most **one** optional data disk at order time; more disks may appear later via reconfiguration.",
				Attributes:          diskAttrs,
			},
			"subnet": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Subnet **name** as returned by the compute API (ManageIQ `cloud_subnets.name`). " +
					"It often matches the **network** display name, e.g. `<location>_<account>_<short_name>`, not the short name you typed in `mistedo_network.name`. " +
					"When the network is managed by `mistedo_network`, pass **`canonical_name`** here.",
				Validators: []validator.String{stringvalidator.LengthBetween(1, 256)},
			},
			"private_ipv4_cidr": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "IPv4 CIDR of the resolved subnet (read-only).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"private_ip_allocation": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Either `auto` (platform assigns an address in the subnet) or `manual` (you must set `private_ipv4_address` inside that subnet).",
				Validators: []validator.String{
					stringvalidator.OneOf("auto", "manual"),
				},
			},
			"private_ipv4_address": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Required when `private_ip_allocation = manual`; must fall inside the subnet CIDR.",
			},
			"security_group_ids": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Firewall group ids (`security_group` in the API — currently the first id is sent if the API accepts one).",
				Validators: []validator.List{
					listvalidator.SizeAtMost(1),
				},
			},
			"pass_auth": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Password auth policy (`pass_auth` in the API).",
			},
			"password": schema.StringAttribute{
				Optional:  true,
				Computed:  true,
				Sensitive: true,
				MarkdownDescription: "Admin password for the order. **Omit** this argument to let the provider generate a password at create time. " +
					"Do not set it to an empty string — use `null` or omit the attribute (empty strings break Terraform's sensitive value rules and plan validation). " +
					"After create, the value is stored in state; refresh keeps it when the argument stays omitted.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 4096),
				},
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"ssh_public_keys": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "SSH public keys; sent as `user_ssh_keys` and the first key as `ssh_key`.",
			},
			"public_remote_access": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "e.g. `[\"22/tcp\"]`.",
			},
			"managed_access": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Platform managed access (`managed_access`).",
			},
			"user_data": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Cloud-init / user data.",
			},
			"lifecycle_state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Service `lifecycle_state` (e.g. `provisioned`).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"region_number": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Derived from the third octet of the subnet IPv4 CIDR for the order payload.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"vlan": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Vlan string sent to the API (`name (name)`).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"instances": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "One entry per VM with boot disk, all data disks, and network interfaces.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: instAttrs,
				},
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *instanceGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Invalid provider data", fmt.Sprintf("Expected a configured Mistedo client, got %T.", req.ProviderData))
		return
	}
	r.client = c
}

func (r *instanceGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider did not supply an API client.")
		return
	}
	var plan instanceGroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sub, err := r.client.ResolveSubnetForInstanceGroup(ctx, plan.Subnet.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Could not resolve subnet", err.Error())
		return
	}

	allocation := strings.ToLower(strings.TrimSpace(plan.PrivateIPAllocation.ValueString()))
	var ipAddr string
	switch allocation {
	case "manual":
		ipAddr = strings.TrimSpace(plan.PrivateIPv4Address.ValueString())
		if ipAddr == "" {
			resp.Diagnostics.AddAttributeError(path.Root("private_ipv4_address"),
				"private_ipv4_address is required", "Set private_ipv4_address when private_ip_allocation is \"manual\".")
			return
		}
		if !client.IPv4InCIDR(ipAddr, sub.Cidr) {
			resp.Diagnostics.AddAttributeError(path.Root("private_ipv4_address"),
				"private_ipv4_address not in subnet CIDR",
				fmt.Sprintf("Address %q must lie within %q (subnet %q).", ipAddr, sub.Cidr, plan.Subnet.ValueString()))
			return
		}
	case "auto":
		if !plan.PrivateIPv4Address.IsNull() && strings.TrimSpace(plan.PrivateIPv4Address.ValueString()) != "" {
			resp.Diagnostics.AddAttributeError(path.Root("private_ipv4_address"),
				"private_ipv4_address must not be set", "Leave private_ipv4_address unset when private_ip_allocation is \"auto\".")
			return
		}
		ipAddr = ""
	default:
		resp.Diagnostics.AddError("Invalid private_ip_allocation", allocation)
		return
	}

	password := strings.TrimSpace(plan.Password.ValueString())
	if password == "" {
		var genErr error
		password, genErr = client.GenerateInstanceGroupPassword()
		if genErr != nil {
			resp.Diagnostics.AddError("Could not generate password", genErr.Error())
			return
		}
	}

	boot, d := diskSpecFromObject(ctx, plan.BootDisk, path.Root("boot_disk"))
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	var dataSize int64
	var dataType string
	if !plan.DataDisk.IsNull() && !plan.DataDisk.IsUnknown() {
		dataDisk, d2 := diskSpecFromObject(ctx, plan.DataDisk, path.Root("data_disk"))
		resp.Diagnostics.Append(d2...)
		if resp.Diagnostics.HasError() {
			return
		}
		dataSize = dataDisk.SizeGib
		dataType = dataDisk.Type
	}

	var sgID string
	if !plan.SecurityGroupIds.IsNull() {
		var ids []string
		resp.Diagnostics.Append(plan.SecurityGroupIds.ElementsAs(ctx, &ids, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if len(ids) > 0 {
			sgID = strings.TrimSpace(ids[0])
		}
	}

	sshKeys := stringListFromTF(ctx, plan.SshPublicKeys, &resp.Diagnostics)
	remote := stringListFromTF(ctx, plan.PublicRemoteAccess, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	sshPrimary := ""
	if len(sshKeys) > 0 {
		sshPrimary = sshKeys[0]
	}

	ord := client.InstanceGroupOrder{
		ServiceName:          plan.Name.ValueString(),
		ServiceDescription:   "",
		TemplateID:           plan.TemplateID.ValueString(),
		DesiredInstanceCount: plan.DesiredInstanceCount.ValueInt64(),
		CPUCores:             plan.CPUCores.ValueInt64(),
		MemoryMiB:            plan.MemoryMiB.ValueInt64(),
		BootDiskSizeGiB:      boot.SizeGib,
		BootDiskType:         boot.Type,
		DataDiskSizeGiB:      dataSize,
		DataDiskType:         dataType,
		SubnetDisplayName:    plan.Subnet.ValueString(),
		ResolvedCIDR:         sub.Cidr,
		ResolvedVlan:         sub.Vlan,
		RegionNumber:         sub.RegionNumber,
		SecurityGroupID:      sgID,
		PassAuth:             plan.PassAuth.ValueString(),
		Password:             password,
		UserSSHKeys:          sshKeys,
		SSHKeyPrimary:        sshPrimary,
		PublicRemoteAccess:   remote,
		ManagedAccess:        "",
		UserData:             "",
		IPAssignPolicy:       allocation,
		IPSubnet:             sub.Cidr,
		IPAddress:            ipAddr,
	}
	if !plan.Description.IsNull() {
		ord.ServiceDescription = plan.Description.ValueString()
	}
	if !plan.ManagedAccess.IsNull() {
		ord.ManagedAccess = plan.ManagedAccess.ValueString()
	}
	if !plan.UserData.IsNull() {
		ord.UserData = plan.UserData.ValueString()
	}

	srID, err := r.client.SubmitInstanceGroupCart(ctx, ord)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not submit instance group order", err)...)
		return
	}
	svcID, err := r.client.PollUntilServiceIDCreated(ctx, srID)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Waiting for service id from service request", err)...)
		return
	}
	svc, err := r.client.WaitInstanceGroupReady(ctx, svcID, int(plan.DesiredInstanceCount.ValueInt64()))
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Waiting for instance group to provision", err)...)
		return
	}
	vms, err := r.client.LoadInstanceGroupVMDetails(ctx, svc)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not load VM details", err)...)
		return
	}

	state := planToStateBase(plan, password)
	state.ID = types.StringValue(svcID)
	state.PrivateIPv4Cidr = types.StringValue(sub.Cidr)
	state.RegionNumber = types.StringValue(sub.RegionNumber)
	state.Vlan = types.StringValue(sub.Vlan)
	state.LifecycleState = types.StringValue(svc.LifecycleState)
	instList, d := instancesToTFList(ctx, svc, vms)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Instances = instList
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *instanceGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider did not supply an API client.")
		return
	}
	var state instanceGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	savedPassword := state.Password

	svc, err := r.client.GetServiceInstanceGroup(ctx, state.ID.ValueString(), "vms,networks")
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.Append(diagAPI("Could not read instance group (service)", err)...)
		return
	}
	sub, err := r.client.ResolveSubnetForInstanceGroup(ctx, state.Subnet.ValueString())
	if err != nil {
		resp.Diagnostics.AddWarning("Could not refresh subnet metadata", err.Error())
	} else {
		state.PrivateIPv4Cidr = types.StringValue(sub.Cidr)
		state.RegionNumber = types.StringValue(sub.RegionNumber)
		state.Vlan = types.StringValue(sub.Vlan)
	}
	state.LifecycleState = types.StringValue(svc.LifecycleState)
	vms, err := r.client.LoadInstanceGroupVMDetails(ctx, svc)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not load VM details", err)...)
		return
	}
	instList, d := instancesToTFList(ctx, svc, vms)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Instances = instList
	state.Password = savedPassword
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *instanceGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider did not supply an API client.")
		return
	}
	var plan, prior instanceGroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !instanceGroupConfigurableMatch(plan, prior) {
		resp.Diagnostics.AddError(
			"Update not supported",
			"The mistedo_instance_group resource has no API for in-place changes to the service. "+
				"To change name, size, disks, subnet, template, or counts, use terraform apply -replace on this resource (or destroy then create). "+
				"Applying is allowed when only computed fields (e.g. instances, lifecycle_state) need refreshing.",
		)
		return
	}

	readReq := resource.ReadRequest{State: req.State}
	readResp := resource.ReadResponse{State: resp.State}
	r.Read(ctx, readReq, &readResp)
	resp.Diagnostics.Append(readResp.Diagnostics...)
}

func instanceGroupConfigurableMatch(plan, prior instanceGroupModel) bool {
	if !plan.ID.Equal(prior.ID) {
		return false
	}
	if !plan.Name.Equal(prior.Name) {
		return false
	}
	if !plan.Description.Equal(prior.Description) {
		return false
	}
	if !plan.TemplateID.Equal(prior.TemplateID) {
		return false
	}
	if !plan.DesiredInstanceCount.Equal(prior.DesiredInstanceCount) {
		return false
	}
	if !plan.CPUCores.Equal(prior.CPUCores) {
		return false
	}
	if !plan.MemoryMiB.Equal(prior.MemoryMiB) {
		return false
	}
	if !plan.BootDisk.Equal(prior.BootDisk) {
		return false
	}
	if !plan.DataDisk.Equal(prior.DataDisk) {
		return false
	}
	if !plan.Subnet.Equal(prior.Subnet) {
		return false
	}
	if !plan.PrivateIPAllocation.Equal(prior.PrivateIPAllocation) {
		return false
	}
	if !plan.PrivateIPv4Address.Equal(prior.PrivateIPv4Address) {
		return false
	}
	if !plan.SecurityGroupIds.Equal(prior.SecurityGroupIds) {
		return false
	}
	if !plan.PassAuth.Equal(prior.PassAuth) {
		return false
	}
	if !plan.Password.Equal(prior.Password) {
		return false
	}
	if !plan.SshPublicKeys.Equal(prior.SshPublicKeys) {
		return false
	}
	if !plan.PublicRemoteAccess.Equal(prior.PublicRemoteAccess) {
		return false
	}
	if !plan.ManagedAccess.Equal(prior.ManagedAccess) {
		return false
	}
	if !plan.UserData.Equal(prior.UserData) {
		return false
	}
	return true
}

func (r *instanceGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		return
	}
	var state instanceGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.RetireService(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not retire instance group", err)...)
	}
}

type diskSpec struct {
	SizeGib int64
	Type    string
}

func diskSpecFromObject(ctx context.Context, obj types.Object, p path.Path) (diskSpec, diag.Diagnostics) {
	var diags diag.Diagnostics
	var m igDiskSpecModel
	diags.Append(obj.As(ctx, &m, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return diskSpec{}, diags
	}
	return diskSpec{SizeGib: m.SizeGib.ValueInt64(), Type: strings.TrimSpace(m.Type.ValueString())}, diags
}

func planToStateBase(plan instanceGroupModel, password string) instanceGroupModel {
	out := plan
	out.Password = types.StringValue(password)
	return out
}

func stringListFromTF(ctx context.Context, list types.List, diags *diag.Diagnostics) []string {
	if list.IsNull() || list.IsUnknown() {
		return nil
	}
	var s []string
	diags.Append(list.ElementsAs(ctx, &s, false)...)
	return s
}

func instancesToTFList(ctx context.Context, svc *client.ServiceInstanceGroup, vms []client.VMInstanceGroupDetail) (types.List, diag.Diagnostics) {
	var outDiags diag.Diagnostics
	elemType := instancesNestedObjectType()
	elems := make([]attr.Value, 0, len(vms))
	for _, vm := range vms {
		bootObj, dataList, d := vmDisksToNested(ctx, vm.Disks)
		outDiags.Append(d...)
		if outDiags.HasError() {
			return types.ListNull(elemType), outDiags
		}
		nicList, d2 := nicListToTF(ctx, nicsForVM(vm.ID, svc.Networks))
		outDiags.Append(d2...)
		if outDiags.HasError() {
			return types.ListNull(elemType), outDiags
		}
		ov, d3 := types.ObjectValue(elemType.AttrTypes, map[string]attr.Value{
			"id":                 types.StringValue(vm.ID),
			"name":               types.StringValue(vm.Name),
			"boot_disk":          bootObj,
			"data_disks":         dataList,
			"network_interfaces": nicList,
		})
		outDiags.Append(d3...)
		if outDiags.HasError() {
			return types.ListNull(elemType), outDiags
		}
		elems = append(elems, ov)
	}
	list, lDiags := types.ListValue(elemType, elems)
	outDiags.Append(lDiags...)
	return list, outDiags
}

func instancesNestedObjectType() types.ObjectType {
	return types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"id":   types.StringType,
			"name": types.StringType,
			"boot_disk": types.ObjectType{AttrTypes: map[string]attr.Type{
				"id": types.StringType, "size_gib": types.Int64Type, "storage_id": types.StringType, "filename": types.StringType,
			}},
			"data_disks": types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{
				"id": types.StringType, "size_gib": types.Int64Type, "storage_id": types.StringType, "filename": types.StringType,
			}}},
			"network_interfaces": types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{
				"subnet": types.StringType, "nic": types.StringType, "ip": types.StringType,
				"hostname": types.StringType, "mac": types.StringType, "type": types.StringType,
			}}},
		},
	}
}

func vmDisksToNested(ctx context.Context, disks []client.VmDiskBrief) (types.Object, types.List, diag.Diagnostics) {
	var outDiags diag.Diagnostics
	diskAttrTypes := map[string]attr.Type{
		"id": types.StringType, "size_gib": types.Int64Type, "storage_id": types.StringType, "filename": types.StringType,
	}
	diskObjType := types.ObjectType{AttrTypes: diskAttrTypes}
	cp := append([]client.VmDiskBrief(nil), disks...)
	client.SortVMDisksByID(cp)
	if len(cp) == 0 {
		boot, d := types.ObjectValue(diskAttrTypes, map[string]attr.Value{
			"id": types.StringValue(""), "size_gib": types.Int64Value(0),
			"storage_id": types.StringValue(""), "filename": types.StringValue(""),
		})
		outDiags.Append(d...)
		empty, d2 := types.ListValue(diskObjType, []attr.Value{})
		outDiags.Append(d2...)
		return boot, empty, outDiags
	}
	b0 := cp[0]
	boot, d := types.ObjectValue(diskAttrTypes, map[string]attr.Value{
		"id": types.StringValue(b0.ID), "size_gib": types.Int64Value(client.DiskSizeGiB(b0.Size)),
		"storage_id": types.StringValue(b0.StorageID), "filename": types.StringValue(b0.Filename),
	})
	outDiags.Append(d...)
	dataElems := make([]attr.Value, 0, len(cp)-1)
	for _, dk := range cp[1:] {
		o, d2 := types.ObjectValue(diskAttrTypes, map[string]attr.Value{
			"id": types.StringValue(dk.ID), "size_gib": types.Int64Value(client.DiskSizeGiB(dk.Size)),
			"storage_id": types.StringValue(dk.StorageID), "filename": types.StringValue(dk.Filename),
		})
		outDiags.Append(d2...)
		dataElems = append(dataElems, o)
	}
	dataList, d3 := types.ListValue(diskObjType, dataElems)
	outDiags.Append(d3...)
	return boot, dataList, outDiags
}

type nicRow struct {
	Subnet, Nic, IP, Hostname, Mac, Type string
}

func nicsForVM(vmID string, nets []client.ServiceNetwork) []nicRow {
	var rows []nicRow
	for _, n := range nets {
		for _, a := range n.Allocations {
			if a.VmID.String() != vmID {
				continue
			}
			rows = append(rows, nicRow{
				Subnet: n.Name, Nic: a.NicName, IP: a.IP, Hostname: a.Hostname,
				Mac: a.Mac, Type: a.Type,
			})
		}
	}
	return rows
}

func nicListToTF(ctx context.Context, rows []nicRow) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elem := types.ObjectType{AttrTypes: map[string]attr.Type{
		"subnet": types.StringType, "nic": types.StringType, "ip": types.StringType,
		"hostname": types.StringType, "mac": types.StringType, "type": types.StringType,
	}}
	elems := make([]attr.Value, 0, len(rows))
	for _, r := range rows {
		o, d := types.ObjectValue(elem.AttrTypes, map[string]attr.Value{
			"subnet": types.StringValue(r.Subnet), "nic": types.StringValue(r.Nic), "ip": types.StringValue(r.IP),
			"hostname": types.StringValue(r.Hostname), "mac": types.StringValue(r.Mac), "type": types.StringValue(r.Type),
		})
		diags.Append(d...)
		elems = append(elems, o)
	}
	return types.ListValue(elem, elems)
}
