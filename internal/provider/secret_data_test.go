// Copyright (c) Ingo Struck
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestSecretDataSourceResource_New(t *testing.T) {
	if NewSecretDataSourceResource() == nil {
		t.Fatal("expected a data source")
	}
}

func TestSecretDataSourceResource_Metadata(t *testing.T) {
	resp := &datasource.MetadataResponse{}
	(&SecretDataSourceResource{}).Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "gopass"}, resp)
	if resp.TypeName != "gopass_secret" {
		t.Errorf("expected type name %q, got %q", "gopass_secret", resp.TypeName)
	}
}

func TestSecretDataSourceResource_Schema(t *testing.T) {
	resp := &datasource.SchemaResponse{}
	(&SecretDataSourceResource{}).Schema(context.Background(), datasource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Schema returned errors: %v", resp.Diagnostics)
	}
	path, ok := resp.Schema.Attributes["path"]
	if !ok || !path.IsRequired() {
		t.Fatal("expected required path attribute")
	}
	value, ok := resp.Schema.Attributes["value"]
	if !ok || !value.IsComputed() || !value.IsSensitive() {
		t.Fatal("expected computed, sensitive value attribute")
	}
}

func TestSecretDataSourceResource_Configure(t *testing.T) {
	r := &SecretDataSourceResource{}
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

func TestSecretDataSourceResource_ConfigureNilData(t *testing.T) {
	resp := &datasource.ConfigureResponse{}
	(&SecretDataSourceResource{}).Configure(context.Background(), datasource.ConfigureRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Configure returned errors: %v", resp.Diagnostics)
	}
}

func TestSecretDataSourceResource_ConfigureInvalidData(t *testing.T) {
	resp := &datasource.ConfigureResponse{}
	(&SecretDataSourceResource{}).Configure(context.Background(), datasource.ConfigureRequest{ProviderData: "invalid"}, resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error for invalid provider data")
	}
}

func secretDataSourceValue(path string) tftypes.Value {
	return tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"path": tftypes.String, "value": tftypes.String,
	}}, map[string]tftypes.Value{
		"path":  tftypes.NewValue(tftypes.String, path),
		"value": tftypes.NewValue(tftypes.String, nil),
	})
}

func TestSecretDataSourceResource_Read(t *testing.T) {
	r := &SecretDataSourceResource{}
	store := newMockStore()
	store.secrets["test/secret"] = newMockSecret("test-password")
	client := NewGopassClient("")
	client.store = store
	r.client = client

	ctx := context.Background()
	schemaResp := &datasource.SchemaResponse{}
	r.Schema(ctx, datasource.SchemaRequest{}, schemaResp)
	stateRaw := tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"path": tftypes.String, "value": tftypes.String,
	}}, nil)
	resp := &datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: stateRaw}}

	r.Read(ctx, datasource.ReadRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: secretDataSourceValue("test/secret")},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Read returned errors: %v", resp.Diagnostics)
	}
	var data SecretModel
	resp.Diagnostics.Append(resp.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		t.Fatalf("failed to decode state: %v", resp.Diagnostics)
	}
	if data.Path.ValueString() != "test/secret" || data.Value.ValueString() != "test-password" {
		t.Errorf("unexpected state: path=%q value=%q", data.Path.ValueString(), data.Value.ValueString())
	}
}

func TestSecretDataSourceResource_ReadError(t *testing.T) {
	r := &SecretDataSourceResource{}
	store := newMockStore()
	store.shouldFail = true
	store.failMsg = "read error"
	client := NewGopassClient("")
	client.store = store
	r.client = client

	ctx := context.Background()
	schemaResp := &datasource.SchemaResponse{}
	r.Schema(ctx, datasource.SchemaRequest{}, schemaResp)
	stateRaw := tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"path": tftypes.String, "value": tftypes.String,
	}}, nil)
	resp := &datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: stateRaw}}

	r.Read(ctx, datasource.ReadRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: secretDataSourceValue("test/secret")},
	}, resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error when the secret cannot be read")
	}
}

func TestSecretDataSourceResource_ReadConfigError(t *testing.T) {
	r := &SecretDataSourceResource{}
	r.client = NewGopassClient("")
	ctx := context.Background()
	schemaResp := &datasource.SchemaResponse{}
	r.Schema(ctx, datasource.SchemaRequest{}, schemaResp)
	wrong := tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"path": tftypes.Number, "value": tftypes.String,
	}}, map[string]tftypes.Value{
		"path":  tftypes.NewValue(tftypes.Number, 123),
		"value": tftypes.NewValue(tftypes.String, nil),
	})
	stateRaw := tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"path": tftypes.String, "value": tftypes.String,
	}}, nil)
	resp := &datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: stateRaw}}

	r.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: wrong}}, resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error for invalid configuration")
	}
}
