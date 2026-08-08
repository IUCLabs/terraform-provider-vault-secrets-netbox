package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &secretBackendRoleResource{}
	_ resource.ResourceWithImportState = &secretBackendRoleResource{}
)

type secretBackendRoleResource struct {
	client *vaultClient
}

type secretBackendRoleModel struct {
	Backend        types.String `tfsdk:"backend"`
	Name           types.String `tfsdk:"name"`
	NetboxUserID   types.Int64  `tfsdk:"netbox_user_id"`
	NetboxUsername types.String `tfsdk:"netbox_username"`
	WriteEnabled   types.Bool   `tfsdk:"write_enabled"`
	Description    types.String `tfsdk:"description"`
	TTL            types.Int64  `tfsdk:"ttl"`
	MaxTTL         types.Int64  `tfsdk:"max_ttl"`
}

func NewSecretBackendRoleResource() resource.Resource {
	return &secretBackendRoleResource{}
}

func (r *secretBackendRoleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secret_backend_role"
}

func (r *secretBackendRoleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a role on the NetBox secrets engine that mints tokens for a pre-existing NetBox service user.",
		Attributes: map[string]schema.Attribute{
			"backend": schema.StringAttribute{
				Description: "Mount path of the NetBox secrets engine.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name of the role.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"netbox_user_id": schema.Int64Attribute{
				Description: "Id of the NetBox service user tokens are minted for. Set this or netbox_username. Computed when resolved from netbox_username.",
				Optional:    true,
				Computed:    true,
			},
			"netbox_username": schema.StringAttribute{
				Description: "Username of the NetBox service user, resolved to an id at write time. Alternative to netbox_user_id.",
				Optional:    true,
			},
			"write_enabled": schema.BoolAttribute{
				Description: "Whether minted tokens permit create/update/delete (default false: read-only).",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"description": schema.StringAttribute{
				Description: "Description applied to each minted NetBox token.",
				Optional:    true,
			},
			"ttl": schema.Int64Attribute{
				Description: "Default lease TTL in seconds for tokens minted from this role (the minted token's NetBox expiry is aligned to the lease).",
				Optional:    true,
			},
			"max_ttl": schema.Int64Attribute{
				Description: "Maximum lease TTL in seconds for tokens minted from this role.",
				Optional:    true,
			},
		},
	}
}

func (r *secretBackendRoleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*vaultClient)
	if !ok {
		resp.Diagnostics.AddError("unexpected provider data type", fmt.Sprintf("got %T", req.ProviderData))
		return
	}
	r.client = client
}

func (r *secretBackendRoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan secretBackendRoleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.write(ctx, rolePath(plan.Backend.ValueString(), plan.Name.ValueString()), roleData(plan)); err != nil {
		resp.Diagnostics.AddError("failed to create netbox role", err.Error())
		return
	}
	resp.Diagnostics.Append(r.readInto(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *secretBackendRoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state secretBackendRoleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	role, err := r.client.read(ctx, rolePath(state.Backend.ValueString(), state.Name.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("failed to read netbox role", err.Error())
		return
	}
	if role == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	applyRoleData(&state, role)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *secretBackendRoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan secretBackendRoleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.write(ctx, rolePath(plan.Backend.ValueString(), plan.Name.ValueString()), roleData(plan)); err != nil {
		resp.Diagnostics.AddError("failed to update netbox role", err.Error())
		return
	}
	resp.Diagnostics.Append(r.readInto(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *secretBackendRoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state secretBackendRoleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.delete(ctx, rolePath(state.Backend.ValueString(), state.Name.ValueString())); err != nil {
		resp.Diagnostics.AddError("failed to delete netbox role", err.Error())
		return
	}
}

func (r *secretBackendRoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	backend, name, ok := splitBackendName(req.ID, "roles")
	if !ok {
		resp.Diagnostics.AddError(
			"invalid import ID",
			fmt.Sprintf("expected \"<backend>/roles/<name>\", got %q", req.ID),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("backend"), backend)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
}

func (r *secretBackendRoleResource) readInto(ctx context.Context, m *secretBackendRoleModel) diag.Diagnostics {
	var diags diag.Diagnostics
	role, err := r.client.read(ctx, rolePath(m.Backend.ValueString(), m.Name.ValueString()))
	if err != nil {
		diags.AddError("failed to read back netbox role", err.Error())
		return diags
	}
	if role == nil {
		diags.AddError("role missing after write", "the role was not found immediately after being written")
		return diags
	}
	applyRoleData(m, role)
	return diags
}

func roleData(m secretBackendRoleModel) map[string]interface{} {
	data := map[string]interface{}{
		"write_enabled": m.WriteEnabled.ValueBool(),
	}
	if !m.NetboxUserID.IsNull() && !m.NetboxUserID.IsUnknown() && m.NetboxUserID.ValueInt64() != 0 {
		data["netbox_user_id"] = m.NetboxUserID.ValueInt64()
	}
	if !m.NetboxUsername.IsNull() && !m.NetboxUsername.IsUnknown() {
		data["netbox_username"] = m.NetboxUsername.ValueString()
	}
	if !m.Description.IsNull() && !m.Description.IsUnknown() {
		data["description"] = m.Description.ValueString()
	}
	if !m.TTL.IsNull() && !m.TTL.IsUnknown() {
		data["ttl"] = m.TTL.ValueInt64()
	}
	if !m.MaxTTL.IsNull() && !m.MaxTTL.IsUnknown() {
		data["max_ttl"] = m.MaxTTL.ValueInt64()
	}
	return data
}

func applyRoleData(m *secretBackendRoleModel, role map[string]interface{}) {
	if n, ok := toInt64(role["netbox_user_id"]); ok {
		m.NetboxUserID = types.Int64Value(n)
	}
	if v, ok := role["netbox_username"].(string); ok && v != "" {
		m.NetboxUsername = types.StringValue(v)
	} else if m.NetboxUsername.IsUnknown() {
		m.NetboxUsername = types.StringNull()
	}
	if b, ok := toBool(role["write_enabled"]); ok {
		m.WriteEnabled = types.BoolValue(b)
	}
	if v, ok := role["description"].(string); ok && v != "" {
		m.Description = types.StringValue(v)
	} else if m.Description.IsUnknown() {
		m.Description = types.StringNull()
	}
	if n, ok := toInt64(role["ttl"]); ok && n != 0 {
		m.TTL = types.Int64Value(n)
	} else if m.TTL.IsUnknown() {
		m.TTL = types.Int64Null()
	}
	if n, ok := toInt64(role["max_ttl"]); ok && n != 0 {
		m.MaxTTL = types.Int64Value(n)
	} else if m.MaxTTL.IsUnknown() {
		m.MaxTTL = types.Int64Null()
	}
}
