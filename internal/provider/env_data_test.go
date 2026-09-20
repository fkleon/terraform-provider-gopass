// Copyright (c) Ingo Struck
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestEnvDataSourceResource_New(t *testing.T) {
	if NewEnvDataSourceResource() == nil {
		t.Fatal("expected a data source")
	}
}

func TestEnvDataSourceResource_Metadata(t *testing.T) {
	r := &EnvDataSourceResource{}
	resp := &datasource.MetadataResponse{}

	r.Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "gopass"}, resp)

	if resp.TypeName != "gopass_env" {
		t.Errorf("expected type name %q, got %q", "gopass_env", resp.TypeName)
	}
}

func TestEnvDataSourceResource_Schema(t *testing.T) {
	r := &EnvDataSourceResource{}
	resp := &datasource.SchemaResponse{}

	r.Schema(context.Background(), datasource.SchemaRequest{}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Schema returned errors: %v", resp.Diagnostics)
	}

	path, ok := resp.Schema.Attributes["path"]
	if !ok || !path.IsRequired() {
		t.Fatal("expected required path attribute")
	}
	credentials, ok := resp.Schema.Attributes["credentials"]
	if !ok || !credentials.IsComputed() || !credentials.IsSensitive() {
		t.Fatal("expected computed, sensitive credentials attribute")
	}
}

func TestEnvDataSourceResource_Configure(t *testing.T) {
	r := &EnvDataSourceResource{}
	client := NewGopassClient("")
	resp := &datasource.ConfigureResponse{}

	r.Configure(context.Background(), datasource.ConfigureRequest{ProviderData: client}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Configure returned errors: %v", resp.Diagnostics)
	}
	if r.client != client {
		t.Fatal("expected provider client to be set")
	}
}

func TestEnvDataSourceResource_ConfigureNilData(t *testing.T) {
	r := &EnvDataSourceResource{}
	resp := &datasource.ConfigureResponse{}

	r.Configure(context.Background(), datasource.ConfigureRequest{}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Configure returned errors: %v", resp.Diagnostics)
	}
}

func TestEnvDataSourceResource_ConfigureInvalidData(t *testing.T) {
	r := &EnvDataSourceResource{}
	resp := &datasource.ConfigureResponse{}

	r.Configure(context.Background(), datasource.ConfigureRequest{ProviderData: "invalid"}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error for invalid provider data")
	}
}

func envDataSourceValue(path string) tftypes.Value {
	return tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"path": tftypes.String, "credentials": tftypes.DynamicPseudoType,
	}}, map[string]tftypes.Value{
		"path":        tftypes.NewValue(tftypes.String, path),
		"credentials": tftypes.NewValue(tftypes.DynamicPseudoType, nil),
	})
}

func TestEnvDataSourceResource_Read(t *testing.T) {
	r := &EnvDataSourceResource{}
	store := newMockStore()
	store.secrets["env/test/REGION"] = newMockSecret("us-east-1")
	store.secrets["env/test/API/v2/KEY"] = newMockSecret("secret")
	client := NewGopassClient("")
	client.store = store
	r.client = client

	ctx := context.Background()
	schemaResp := &datasource.SchemaResponse{}
	r.Schema(ctx, datasource.SchemaRequest{}, schemaResp)
	configValue := envDataSourceValue("env/test")
	stateValue := tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"path": tftypes.String, "credentials": tftypes.DynamicPseudoType,
	}}, nil)

	resp := &datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: stateValue}}
	r.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: configValue}}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Read returned errors: %v", resp.Diagnostics)
	}

	var data EnvModel
	resp.Diagnostics.Append(resp.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		t.Fatalf("failed to decode state: %v", resp.Diagnostics)
	}
	if data.Path.ValueString() != "env/test" {
		t.Errorf("expected path %q, got %q", "env/test", data.Path.ValueString())
	}
	if data.Credentials.IsNull() || data.Credentials.IsUnknown() {
		t.Fatal("expected credentials to be populated")
	}
	credentials, ok := data.Credentials.UnderlyingValue().(types.Object)
	if !ok {
		t.Fatalf("expected credentials object, got %T", data.Credentials.UnderlyingValue())
	}
	if len(credentials.Attributes()) != 2 {
		t.Errorf("expected 2 credentials, got %d", len(credentials.Attributes()))
	}
	region, ok := credentials.Attributes()["REGION"].(types.String)
	if !ok || region.ValueString() != "us-east-1" {
		t.Errorf("expected REGION to be %q, got %#v", "us-east-1", credentials.Attributes()["REGION"])
	}
	api, ok := credentials.Attributes()["API"].(types.Object)
	if !ok {
		t.Fatalf("expected API to be an object, got %T", credentials.Attributes()["API"])
	}
	v2, ok := api.Attributes()["v2"].(types.Object)
	if !ok {
		t.Fatalf("expected API.v2 to be an object, got %T", api.Attributes()["v2"])
	}
	key, ok := v2.Attributes()["KEY"].(types.String)
	if !ok || key.ValueString() != "secret" {
		t.Errorf("expected API.v2.KEY to be %q, got %#v", "secret", v2.Attributes()["KEY"])
	}
}

func TestEnvDataSourceResource_ReadEmpty(t *testing.T) {
	r := &EnvDataSourceResource{}
	client := NewGopassClient("")
	client.store = newMockStore()
	r.client = client

	ctx := context.Background()
	schemaResp := &datasource.SchemaResponse{}
	r.Schema(ctx, datasource.SchemaRequest{}, schemaResp)
	stateValue := tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"path": tftypes.String, "credentials": tftypes.DynamicPseudoType,
	}}, nil)
	resp := &datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: stateValue}}

	r.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: envDataSourceValue("empty/path")}}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Read returned errors: %v", resp.Diagnostics)
	}
	if resp.Diagnostics.WarningsCount() == 0 {
		t.Fatal("expected a warning when no secrets are found")
	}
}

func TestEnvDataSourceResource_ReadError(t *testing.T) {
	r := &EnvDataSourceResource{}
	store := newMockStore()
	store.shouldFail = true
	store.failMsg = "list error"
	client := NewGopassClient("")
	client.store = store
	r.client = client

	ctx := context.Background()
	schemaResp := &datasource.SchemaResponse{}
	r.Schema(ctx, datasource.SchemaRequest{}, schemaResp)
	stateValue := tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"path": tftypes.String, "credentials": tftypes.DynamicPseudoType,
	}}, nil)
	resp := &datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: stateValue}}

	r.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: envDataSourceValue("env/test")}}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error when secrets cannot be read")
	}
}

func TestEnvDataSourceResource_ReadConfigError(t *testing.T) {
	r := &EnvDataSourceResource{}
	r.client = NewGopassClient("")
	ctx := context.Background()
	schemaResp := &datasource.SchemaResponse{}
	r.Schema(ctx, datasource.SchemaRequest{}, schemaResp)
	wrong := tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"path": tftypes.Number, "credentials": tftypes.DynamicPseudoType,
	}}, map[string]tftypes.Value{
		"path":        tftypes.NewValue(tftypes.Number, 123),
		"credentials": tftypes.NewValue(tftypes.DynamicPseudoType, nil),
	})
	stateValue := tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"path": tftypes.String, "credentials": tftypes.DynamicPseudoType,
	}}, nil)
	resp := &datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: stateValue}}

	r.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: wrong}}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error for invalid configuration")
	}
}
