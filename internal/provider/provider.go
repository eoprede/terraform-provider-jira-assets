// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"log"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/ctreminiom/go-atlassian/assets"
	"github.com/ctreminiom/go-atlassian/pkg/infra/models"
)

// Ensure the implementation satisfies the expected interfaces.
var _ provider.Provider = &JiraAssetsProvider{}

// New is a helper function to simplify provider server and testing implementation.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &JiraAssetsProvider{
			version: version,
		}
	}
}

// JiraAssetsProvider defines the provider implementation.
type JiraAssetsProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// JiraAssetsProviderModel describes the provider data model.
type JiraAssetsProviderModel struct {
	WorkspaceId    types.String `tfsdk:"workspace_id"`
	User           types.String `tfsdk:"user"`
	Password       types.String `tfsdk:"password"`
	ObjectSchemaId types.String `tfsdk:"object_schema_id"`
	IgnoreKeys     []string     `tfsdk:"ignore_keys"`
}

// Some structures to hopefully pass schema to provider
type Schema struct {
	ID    string
	Name  string
	Types []Type
}

type Type struct {
	ID         string
	Name       string
	Attributes []Attribute
}

type Attribute struct {
	ID   string
	Name string
}

// JiraAssetsProviderClient describes client and worksapceId.
type JiraAssetsProviderClient struct {
	client                 *assets.Client
	workspaceId            string
	objectschemaId         string
	ignoreKeys             []string
	objectSchemaTypes      []*models.ObjectTypeScheme
	objectSchemaAttributes map[string][]*models.ObjectTypeAttributeScheme
}

func getObjectSchemaAttributes(asset *assets.Client, workSpaceID string, objectTypes []*models.ObjectTypeScheme) map[string][]*models.ObjectTypeAttributeScheme {
	ret := make(map[string][]*models.ObjectTypeAttributeScheme)
	options := &models.ObjectTypeAttributesParamsScheme{
		OnlyValueEditable:       true,
		OrderByName:             false,
		Query:                   "",
		IncludeValuesExist:      false,
		ExcludeParentAttributes: false,
		IncludeChildren:         true,
		OrderByRequired:         false,
	}
	for _, objectType := range objectTypes {
		attributes, response, err := asset.ObjectType.Attributes(context.Background(), workSpaceID, objectType.Id, options)
		if err != nil {
			if response != nil {
				log.Println(response.Bytes.String())
				log.Println("Endpoint:", response.Endpoint)
			}
			log.Fatal(err)
		}
		ret[objectType.Id] = attributes
	}
	return ret
}

func getObjectSchemaObjectTypes(asset *assets.Client, workSpaceID string, objsectSchemaID string) []*models.ObjectTypeScheme {
	schema, response, err := asset.ObjectSchema.ObjectTypes(context.Background(), workSpaceID, objsectSchemaID, false)
	if err != nil {
		if response != nil {
			log.Println(response.Bytes.String())
			log.Println("Endpoint:", response.Endpoint)
		}
		log.Fatal(err)
	}
	return schema
}

func (p *JiraAssetsProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "jiraassets"
	resp.Version = p.version
}

func (p *JiraAssetsProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Terraform provider for Jira Assets.",
		Attributes: map[string]schema.Attribute{
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "Workspace Id of the Assets instance.",
				Optional:            true,
			},
			"user": schema.StringAttribute{
				MarkdownDescription: "Username of an admin or service account with access to the Jira API.",
				Optional:            true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "Personal access token for the admin or service account.",
				Optional:            true,
				Sensitive:           true,
			},
			"object_schema_id": schema.StringAttribute{
				MarkdownDescription: "ID of the object schema to use.",
				Optional:            true,
			},
			"ignore_keys": schema.ListAttribute{
				MarkdownDescription: "List of keys to ignore when creating resources.",
				ElementType:         types.StringType,
				Optional:            true,
			},
		},
	}
}

func (p *JiraAssetsProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	tflog.Info(ctx, "Configuring Jira Assets provider")

	var config JiraAssetsProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// If practitioner provided a configuration value for any of the attributes, it must be a known value.

	if config.WorkspaceId.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("workspaceId"),
			"Unknown Assets Workspace Id",
			"The provider cannot create the Assets API client as there is an unknown configuration value for the Assets API workspace Id. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the JIRAASSETS_WORKSPACE_ID environment variable.",
		)
	}

	if config.User.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("user"),
			"Unknown Assets User",
			"The provider cannot create the Assets API client as there is an unknown configuration value for the Assets API user. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the JIRAASSETS_USER environment variable.",
		)
	}

	if config.Password.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("password"),
			"Unknown Assets Password",
			"The provider cannot create the Assets API client as there is an unknown configuration value for the Assets API password. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the JIRAASSETS_PASSWORD environment variable.",
		)
	}

	if config.ObjectSchemaId.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("objectschemaId"),
			"Unknown Asset objectschemaId",
			"The provider cannot create the Assets API client as there is an unknown configuration value for the objectschemaId. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the JIRAASSETS_OBJECTSCHEMA_ID environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Default values to environment variables, but override with Terraform configuration value if set.

	workspaceId := os.Getenv("JIRAASSETS_WORKSPACE_ID")
	user := os.Getenv("JIRAASSETS_USER")
	password := os.Getenv("JIRAASSETS_PASSWORD")
	objectschemaId := os.Getenv("JIRAASSETS_OBJECTSCHEMA_ID")

	if !config.WorkspaceId.IsNull() {
		workspaceId = config.WorkspaceId.ValueString()
	}

	if !config.User.IsNull() {
		user = config.User.ValueString()
	}

	if !config.Password.IsNull() {
		password = config.Password.ValueString()
	}

	if !config.ObjectSchemaId.IsNull() {
		objectschemaId = config.ObjectSchemaId.ValueString()
	}

	// If any of the expected configurations are missing, return errors with provider-specific guidance.

	if workspaceId == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("workspaceId"),
			"Missing Assets API Workspace Id",
			"The provider cannot create the Assets API client as there is a missing or empty value for the Assets API workspace Id. "+
				"Set the host value in the configuration or use the JIRAASSETS_WORKSPACE_ID environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if user == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("user"),
			"Missing Assets API User",
			"The provider cannot create the Assets API client as there is a missing or empty value for the Assets API username. "+
				"Set the user value in the configuration or use the JIRAASSETS_USER environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if password == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("password"),
			"Missing Assets API Password",
			"The provider cannot create the Assets API client as there is a missing or empty value for the Assets API password. "+
				"Set the password value in the configuration or use the JIRAASSETS_PASSWORD environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "jiraassets_workspace_id", workspaceId)
	ctx = tflog.SetField(ctx, "jiraassets_user", user)
	ctx = tflog.SetField(ctx, "jiraassets_password", password)
	ctx = tflog.SetField(ctx, "jiraassets_objectschemaId", objectschemaId)
	ctx = tflog.MaskFieldValuesWithFieldKeys(ctx, "jiraassets_password")

	tflog.Debug(ctx, "Creating HashiCups client")

	// create the Jira Assets client
	client, err := assets.New(nil, "")

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create Assets client",
			"An unexpected error occurred when creating the Assets API client. Error: "+err.Error(),
		)
	}

	// add authentication headers to the client, workspaceId is added to each request
	client.Auth.SetBasicAuth(user, password)

	// build shcme and attribute mappings
	objectSchemaTypes := getObjectSchemaObjectTypes(client, workspaceId, objectschemaId)
	objectSchemaAttributes := getObjectSchemaAttributes(client, workspaceId, objectSchemaTypes)

	// add workspaceId to response to be used by resources and data sources
	providerClient := JiraAssetsProviderClient{
		client:                 client,
		workspaceId:            workspaceId,
		objectschemaId:         objectschemaId,
		ignoreKeys:             config.IgnoreKeys,
		objectSchemaTypes:      objectSchemaTypes,
		objectSchemaAttributes: objectSchemaAttributes,
	}

	resp.DataSourceData = providerClient
	resp.ResourceData = providerClient

	tflog.Info(ctx, "Configured Jira Assets client", map[string]any{"success": true})
}

func (p *JiraAssetsProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewObjectResource,
	}
}

func (p *JiraAssetsProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewObjectSchemaDataSource,
	}
}
