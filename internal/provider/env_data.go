// Copyright (c) Ingo Struck
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure implementation satisfies interface.
var _ datasource.DataSource = &EnvDataSourceResource{}

// EnvDataSourceResource reads a subtree from gopass as environment variables.
type EnvDataSourceResource struct {
	EnvEphemeralResource
}

// NewEnvDataSourceResource creates a new instance.
func NewEnvDataSourceResource() datasource.DataSource {
	return &EnvDataSourceResource{}
}

func (r *EnvDataSourceResource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_env"
}

func (r *EnvDataSourceResource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Reads all secrets under a path as a nested object structure (environment variable style).",
		MarkdownDescription: envMarkdownDescription,

		Attributes: map[string]schema.Attribute{
			"path": schema.StringAttribute{
				Description:         "Path prefix in the gopass store (e.g., 'env/terraform/scaleway/acme').",
				MarkdownDescription: "Path prefix in the gopass store (e.g., `env/terraform/scaleway/acme`).",
				Required:            true,
			},
			"credentials": schema.DynamicAttribute{
				Description:         "Object with secret names as attributes (accessible via dot-notation).",
				MarkdownDescription: "Object with secret names as attributes (accessible via dot-notation).",
				Computed:            true,
				Sensitive:           true,
			},
		},
	}
}

func (r *EnvDataSourceResource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	client, err := configureGopassClient(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError("Unexpected Provider Data", err.Error())
		return
	}
	r.client = client
}

//nolint:gocritic // Terraform framework interface requirement.
func (r *EnvDataSourceResource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data EnvModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	basePath := data.Path.ValueString()

	tflog.Debug(ctx, "Reading env secrets from gopass", map[string]interface{}{
		"path": basePath,
	})

	// Use native gopass library (now returns recursive/nested paths).
	dynamicValue, count, err := readEnvCredentials(ctx, r.client, basePath)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to read secrets",
			fmt.Sprintf("Could not read secrets under path %q: %s", basePath, err.Error()),
		)
		return
	}

	if count == 0 {
		resp.Diagnostics.AddWarning(
			"No secrets found",
			fmt.Sprintf("No secrets found under path %q", basePath),
		)
	}

	data.Credentials = dynamicValue

	// Set result
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

	tflog.Debug(ctx, "Successfully read env secrets from gopass", map[string]interface{}{
		"path":  basePath,
		"count": count,
	})
}
