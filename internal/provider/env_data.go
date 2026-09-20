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

// Ensure implementation satisfies interface.
var _ datasource.DataSource = &EnvDataSourceResource{}

// EnvDataSourceResource reads a subtree from gopass as environment variables.
type EnvDataSourceResource struct {
	//client *GopassClient
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
		Description: "Reads all secrets under a path as a nested object structure (environment variable style).",
		MarkdownDescription: `
Reads all secrets under a path as a nested object structure, using the native gopass library.

Each secret under the path becomes accessible via dot-notation. The secret's first line becomes the value.
Supports both flat and nested/deep path structures.

This is ideal for reading credential sets with hierarchical organization:

` + "```" + `
env/terraform/scaleway/acme/
├── SCW_ACCESS_KEY
├── SCW_SECRET_KEY
├── SCW_DEFAULT_PROJECT_ID
└── API/
    └── v2/
        ├── ACCESS_KEY
        └── SECRET_KEY
` + "```" + `

## Example Usage

**Flat paths (immediate children):**

` + "```hcl" + `
ephemeral "gopass_env" "scaleway" {
  path = "env/terraform/scaleway/acme"
}

provider "scaleway" {
  access_key = ephemeral.gopass_env.scaleway.credentials.SCW_ACCESS_KEY
  secret_key = ephemeral.gopass_env.scaleway.credentials.SCW_SECRET_KEY
  project_id = ephemeral.gopass_env.scaleway.credentials.SCW_DEFAULT_PROJECT_ID
}
` + "```" + `

**Nested paths (deep hierarchies):**

` + "```hcl" + `
ephemeral "gopass_env" "aws" {
  path = "env/terraform/aws"
}

provider "aws" {
  region     = ephemeral.gopass_env.aws.credentials.REGION
  # Access nested paths: API/v2/ACCESS_KEY becomes credentials.API.v2.ACCESS_KEY
  access_key = ephemeral.gopass_env.aws.credentials.API.v2.ACCESS_KEY
  secret_key = ephemeral.gopass_env.aws.credentials.API.v2.SECRET_KEY
}
` + "```" + `

## Notes

- **Recursive**: All secrets under the path are included, regardless of depth
- Each secret's first line is used as the value (gopass password convention)
- Nested paths use dot-notation: ` + "`API/v2/KEY`" + ` becomes ` + "`credentials.API.v2.KEY`" + `
- Supports mixed flat and nested structures in the same tree
- No subprocess spawning - direct library access for better performance
`,

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
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*GopassClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Provider Data",
			fmt.Sprintf("Expected *GopassClient, got: %T", req.ProviderData),
		)
		return
	}

	r.client = client
}

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

	// Use native gopass library (now returns recursive/nested paths)
	values, err := r.client.GetEnvSecrets(ctx, basePath)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to read secrets",
			fmt.Sprintf("Could not read secrets under path %q: %s", basePath, err.Error()),
		)
		return
	}

	if len(values) == 0 {
		resp.Diagnostics.AddWarning(
			"No secrets found",
			fmt.Sprintf("No secrets found under path %q", basePath),
		)
	}

	// Build nested object structure from slash-separated paths
	// This allows accessing "API/v2/ACCESS_KEY" as credentials.API.v2.ACCESS_KEY
	objValue := buildNestedObject(values)

	// Convert to dynamic
	dynamicValue := types.DynamicValue(objValue)
	data.Credentials = dynamicValue

	// Set result
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

	tflog.Debug(ctx, "Successfully read env secrets from gopass", map[string]interface{}{
		"path":  basePath,
		"count": len(values),
	})
}
