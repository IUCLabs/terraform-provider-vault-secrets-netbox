package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &secretBackendResource{}
	_ resource.ResourceWithImportState = &secretBackendResource{}
)

const defaultPluginType = "vault-plugin-secrets-netbox"

type secretBackendResource struct {
	client *vaultClient
}

type secretBackendModel struct {
	Path                  types.String `tfsdk:"path"`
	Plugin                types.String `tfsdk:"plugin"`
	Description           types.String `tfsdk:"description"`
	NetboxURL             types.String `tfsdk:"netbox_url"`
	Token                 types.String `tfsdk:"token"`
	TokenVersion          types.Int64  `tfsdk:"token_version"`
	AdminUserID           types.Int64  `tfsdk:"admin_user_id"`
	AdminTokenID          types.Int64  `tfsdk:"admin_token_id"`
	CACert                types.String `tfsdk:"ca_cert"`
	TLSSkipVerify         types.Bool   `tfsdk:"tls_skip_verify"`
	RequestTimeoutSeconds types.Int64  `tfsdk:"request_timeout_seconds"`
}

func NewSecretBackendResource() resource.Resource {
	return &secretBackendResource{}
}

func (r *secretBackendResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secret_backend"
}

func (r *secretBackendResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Mounts the NetBox secrets engine and writes its connection config, including the seeded admin token.",
		Attributes: map[string]schema.Attribute{
			"path": schema.StringAttribute{
				Description: "Mount path for the NetBox secrets engine (e.g. \"netbox\").",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"plugin": schema.StringAttribute{
				Description: "Name of the plugin in the plugin catalog. Defaults to \"vault-plugin-secrets-netbox\".",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(defaultPluginType),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Description: "Human-readable description of the mount.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"netbox_url": schema.StringAttribute{
				Description: "Base URL of the NetBox server (e.g. https://netbox.example.com).",
				Required:    true,
			},
			"token": schema.StringAttribute{
				Description: "Seeded NetBox admin API token. Write-only on the backend; not read back. Omit to manage it out-of-band (e.g. via config/rotate).",
				Optional:    true,
				Sensitive:   true,
			},
			"token_version": schema.Int64Attribute{
				Description: "NetBox token version to request when minting (2 by default; use 1 if NetBox has no API_TOKEN_PEPPERS).",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(2),
			},
			"admin_user_id": schema.Int64Attribute{
				Description: "NetBox user id of the seeded admin token (computed; the engine auto-discovers/maintains it for rotation).",
				Computed:    true,
			},
			"admin_token_id": schema.Int64Attribute{
				Description: "NetBox token id of the seeded admin token (computed; maintained by the engine across rotations).",
				Computed:    true,
			},
			"ca_cert": schema.StringAttribute{
				Description: "PEM CA certificate that signed the NetBox server's TLS certificate.",
				Optional:    true,
			},
			"tls_skip_verify": schema.BoolAttribute{
				Description: "Skip TLS verification of the NetBox server (not recommended).",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"request_timeout_seconds": schema.Int64Attribute{
				Description: "HTTP timeout in seconds for calls from the plugin to NetBox.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(30),
			},
		},
	}
}

func (r *secretBackendResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *secretBackendResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan secretBackendModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	mountPath := strings.Trim(plan.Path.ValueString(), "/")

	err := r.client.enableMount(ctx, mountPath, plan.Plugin.ValueString(), plan.Description.ValueString(), mountConfig{})
	if err != nil {
		if isMountAlreadyExists(err) {
			resp.Diagnostics.AddError(
				"mount path already in use",
				fmt.Sprintf("A secrets engine is already mounted at %q. Import it or choose another path.", mountPath),
			)
			return
		}
		resp.Diagnostics.AddError("failed to enable netbox secrets engine", err.Error())
		return
	}

	if err := r.client.write(ctx, configPath(mountPath), r.configData(plan)); err != nil {
		// Roll back the mount so we don't leave a half-configured engine.
		_ = r.client.disableMount(ctx, mountPath)
		resp.Diagnostics.AddError("failed to write netbox config", err.Error())
		return
	}

	plan.Path = types.StringValue(mountPath)
	r.applyConfigRead(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *secretBackendResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state secretBackendModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	mountPath := strings.Trim(state.Path.ValueString(), "/")

	mount, err := r.client.mountInfo(ctx, mountPath)
	if err != nil {
		resp.Diagnostics.AddError("failed to read mount", err.Error())
		return
	}
	if mount == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	state.Description = types.StringValue(mount.Description)
	if mount.Type != "" {
		state.Plugin = types.StringValue(mount.Type)
	}

	r.applyConfigRead(ctx, &state, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *secretBackendResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state secretBackendModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	mountPath := strings.Trim(state.Path.ValueString(), "/")

	if !plan.Description.Equal(state.Description) {
		if err := r.client.tuneMount(ctx, mountPath, plan.Description.ValueString(), mountConfig{}); err != nil {
			resp.Diagnostics.AddError("failed to tune mount description", err.Error())
			return
		}
	}

	if err := r.client.write(ctx, configPath(mountPath), r.configData(plan)); err != nil {
		resp.Diagnostics.AddError("failed to update netbox config", err.Error())
		return
	}

	plan.Path = types.StringValue(mountPath)
	r.applyConfigRead(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *secretBackendResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state secretBackendModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.disableMount(ctx, strings.Trim(state.Path.ValueString(), "/")); err != nil {
		resp.Diagnostics.AddError("failed to disable netbox secrets engine", err.Error())
		return
	}
}

func (r *secretBackendResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("path"), req, resp)
}

// configData builds the write payload for config. The seeded admin token is only
// sent when set (it is never read back), and admin_user_id/admin_token_id are
// deliberately omitted so the engine keeps the ids it maintains across rotations.
func (r *secretBackendResource) configData(m secretBackendModel) map[string]interface{} {
	data := map[string]interface{}{
		"netbox_url":              m.NetboxURL.ValueString(),
		"token_version":           m.TokenVersion.ValueInt64(),
		"tls_skip_verify":         m.TLSSkipVerify.ValueBool(),
		"request_timeout_seconds": m.RequestTimeoutSeconds.ValueInt64(),
	}
	if !m.Token.IsNull() && !m.Token.IsUnknown() {
		data["token"] = m.Token.ValueString()
	}
	if !m.CACert.IsNull() && !m.CACert.IsUnknown() {
		data["ca_cert"] = m.CACert.ValueString()
	}
	return data
}

// applyConfigRead reads config back and populates the computed/derived fields.
// token and ca_cert are never returned by the backend, so their state is
// preserved as-is.
func (r *secretBackendResource) applyConfigRead(ctx context.Context, m *secretBackendModel, diags *diag.Diagnostics) {
	cfg, err := r.client.read(ctx, configPath(strings.Trim(m.Path.ValueString(), "/")))
	if err != nil {
		diags.AddError("failed to read netbox config", err.Error())
		return
	}
	if cfg == nil {
		return
	}
	if v, ok := cfg["netbox_url"].(string); ok {
		m.NetboxURL = types.StringValue(v)
	}
	if n, ok := toInt64(cfg["token_version"]); ok {
		m.TokenVersion = types.Int64Value(n)
	}
	if n, ok := toInt64(cfg["admin_user_id"]); ok {
		m.AdminUserID = types.Int64Value(n)
	} else {
		m.AdminUserID = types.Int64Value(0)
	}
	if n, ok := toInt64(cfg["admin_token_id"]); ok {
		m.AdminTokenID = types.Int64Value(n)
	} else {
		m.AdminTokenID = types.Int64Value(0)
	}
	if b, ok := toBool(cfg["tls_skip_verify"]); ok {
		m.TLSSkipVerify = types.BoolValue(b)
	}
	if n, ok := toInt64(cfg["request_timeout_seconds"]); ok {
		m.RequestTimeoutSeconds = types.Int64Value(n)
	}
}
