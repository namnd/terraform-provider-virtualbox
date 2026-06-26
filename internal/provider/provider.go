// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/namnd/terraform-provider-virtualbox/internal/vboxmanage"
)

// Ensure VirtualboxProvider satisfies various provider interfaces.
var _ provider.Provider = &VirtualboxProvider{}
var _ provider.ProviderWithFunctions = &VirtualboxProvider{}
var _ provider.ProviderWithEphemeralResources = &VirtualboxProvider{}
var _ provider.ProviderWithActions = &VirtualboxProvider{}

// VirtualboxProvider defines the provider implementation.
type VirtualboxProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// VirtualboxProviderModel describes the provider data model.
type VirtualboxProviderModel struct{}

func (p *VirtualboxProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "virtualbox"
	resp.Version = p.version
}

func (p *VirtualboxProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{},
	}
}

func (p *VirtualboxProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data VirtualboxProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	client, err := vboxmanage.New()
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create VBoxManage client",
			fmt.Sprintf("Could not initialize VBoxManage client: %s", err),
		)
	}

	if _, err := client.Version(ctx); err != nil {
		resp.Diagnostics.AddError(
			"Unable to connect to VirtualBox",
			fmt.Sprintf("Could not verify VBoxManage installation: %s", err),
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *VirtualboxProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewDiskResource,
		NewVMIPAddressResource,
		NewVMResource,
		NewVMStorageAttachmentResource,
	}
}

func (p *VirtualboxProvider) EphemeralResources(ctx context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{}
}

func (p *VirtualboxProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

func (p *VirtualboxProvider) Functions(ctx context.Context) []func() function.Function {
	return []func() function.Function{}
}

func (p *VirtualboxProvider) Actions(ctx context.Context) []func() action.Action {
	return []func() action.Action{}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &VirtualboxProvider{
			version: version,
		}
	}
}
