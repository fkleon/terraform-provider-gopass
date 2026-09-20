// Copyright (c) Ingo Struck
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

const envMarkdownDescription = `
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
`

func configureGopassClient(providerData any) (*GopassClient, error) {
	if providerData == nil {
		return nil, nil
	}

	client, ok := providerData.(*GopassClient)
	if !ok {
		return nil, fmt.Errorf("expected *GopassClient, got: %T", providerData)
	}
	return client, nil
}

func readEnvCredentials(ctx context.Context, client *GopassClient, path string) (dynamic types.Dynamic, count int, err error) {
	values, err := client.GetEnvSecrets(ctx, path)
	if err != nil {
		return types.DynamicNull(), 0, err
	}
	return types.DynamicValue(buildNestedObject(values)), len(values), nil
}
