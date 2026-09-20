// Copyright (c) Ingo Struck
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ datasource.DataSource = &SecretDataSourceResource{}

// SecretDataSourceResource reads a single secret from gopass into state.
type SecretDataSourceResource struct {
	SecretEphemeralResource
}

// NewSecretDataSourceResource creates a new data source.
func NewSecretDataSourceResource() datasource.DataSource {
	return &SecretDataSourceResource{}
}

func (r *SecretDataSourceResource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secret"
}

func (r *SecretDataSourceResource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads a single secret value from the gopass store into Terraform state.",
		MarkdownDescription: `
Reads a single secret value from the gopass store using the native gopass library.

The result is marked sensitive, but data source values can be stored in Terraform state.
Use ` + "`ephemeral \"gopass_secret\"`" + ` when the secret must not be persisted.

## Example Usage

` + "```hcl" + `
data "gopass_secret" "api_key" {
  path = "services/api/token"
}

# Use the sensitive secret value
data.gopass_secret.api_key.value
` + "```" + `

## GPG/Hardware Token

If your gopass store is encrypted with a hardware token (YubiKey, Nitrokey, etc.),
you will be prompted for PIN entry and/or touch confirmation during each
Terraform operation that accesses the secret.
`,
		Attributes: map[string]schema.Attribute{
			"path": schema.StringAttribute{
				Description:         "Path to the secret in the gopass store (e.g., 'infrastructure/db/password').",
				MarkdownDescription: "Path to the secret in the gopass store (e.g., `infrastructure/db/password`).",
				Required:            true,
			},
			"value": schema.StringAttribute{
				Description:         "The secret value (password/first line of the secret).",
				MarkdownDescription: "The secret value (password/first line of the secret).",
				Computed:            true,
				Sensitive:           true,
			},
		},
	}
}

func (r *SecretDataSourceResource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	client, err := configureGopassClient(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError("Unexpected Provider Data", err.Error())
		return
	}
	r.client = client
}

//nolint:gocritic // Terraform framework interface requirement.
func (r *SecretDataSourceResource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data SecretModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := data.Path.ValueString()
	tflog.Debug(ctx, "Reading secret from gopass", map[string]interface{}{"path": path})

	value, err := r.client.GetSecret(ctx, path)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to read secret",
			fmt.Sprintf("Could not read secret at path %q: %s", path, err.Error()),
		)
		return
	}

	data.Value = types.StringValue(value)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

	tflog.Debug(ctx, "Successfully read secret from gopass", map[string]interface{}{"path": path})
}
