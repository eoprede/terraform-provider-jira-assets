package provider

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/ctreminiom/go-atlassian/assets"
	"github.com/ctreminiom/go-atlassian/pkg/infra/models"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &objectResource{}
	_ resource.ResourceWithConfigure   = &objectResource{}
	_ resource.ResourceWithImportState = &objectResource{}
)

// NewObjectResource is a helper function to simplify the provider implementation.
func NewObjectResource() resource.Resource {
	return &objectResource{}
}

// objectResource is the resource implementation.
type objectResource struct {
	client                 *assets.Client
	workspaceId            string
	objectschemaId         string
	ignoreKeys             []string
	objectSchemaTypes      []*models.ObjectTypeScheme
	objectSchemaAttributes []*models.ObjectTypeAttributeScheme
	configStatusType       *[]StatusTypeMetadata
}

// Metadata returns the resource type name.
func (r *objectResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	// resp.TypeName = req.ProviderTypeName + "_object"
	resp.TypeName = "jiraassets_object"
}

type objectResourceModel struct {

	// avatar
	// objectType
	// hasAvatar
	// timestamp
	// attributes
	// _links

	WorkspaceId types.String `tfsdk:"workspace_id"`
	GlobalId    types.String `tfsdk:"global_id"`
	Id          types.String `tfsdk:"id"`
	Label       types.String `tfsdk:"label"`
	ObjectKey   types.String `tfsdk:"object_key"`
	Created     types.String `tfsdk:"created"`
	Updated     types.String `tfsdk:"updated"`
	HasAvatar   types.Bool   `tfsdk:"has_avatar"`

	Type       types.String `tfsdk:"type"`
	Attributes types.Map    `tfsdk:"attributes"`
	AvatarUuid types.String `tfsdk:"avatar_uuid"`
}

// type objectAttrResourceModel struct {
// 	AttrType  types.String `tfsdk:"attr_type"`
// 	AttrValue types.String `tfsdk:"attr_value"`
// }

func getObjectTypeByName(objName string, schema []*models.ObjectTypeScheme) *models.ObjectTypeScheme {
	for _, obj := range schema {
		if obj.Name == objName {
			return obj
		}
	}
	return &models.ObjectTypeScheme{}
}

func getObjectAttributeByName(objName string, objectType string, schema []*models.ObjectTypeAttributeScheme) *models.ObjectTypeAttributeScheme {
	for _, obj := range schema {
		if obj.Name == objName && obj.ObjectType.Name == objectType {
			return obj
		}
	}
	return &models.ObjectTypeAttributeScheme{}
}

func getAttributeValue(attr *models.ObjectAttributeScheme, statusType *[]StatusTypeMetadata) (string, error) {
	switch attr.ObjectTypeAttribute.Type {
	case 1:
		return attr.ObjectAttributeValues[0].SearchValue, nil
	case 0:
		return attr.ObjectAttributeValues[0].Value, nil
	case 7:
		return getConfigStatusNameByID(attr.ObjectAttributeValues[0].Status.ID, statusType), nil
	default:
		return "", fmt.Errorf("unsupported attribute type: %d", attr.ObjectTypeAttribute.Type)
	}
}

func returnAttributePayloadValue(name string, value string, objectType string, objectSchemaAttributes []*models.ObjectTypeAttributeScheme, statusType *[]StatusTypeMetadata) (*models.ObjectPayloadAttributeScheme, error) {
	attrSchema := getObjectAttributeByName(name, objectType, objectSchemaAttributes)
	var err error
	val := value
	if attrSchema.Type == 7 {
		val, err = getConfigStatusIDByName(value, statusType)
		if err != nil {
			return nil, err
		}
	}
	return &models.ObjectPayloadAttributeScheme{
		ObjectTypeAttributeID: attrSchema.ID,
		ObjectAttributeValues: []*models.ObjectPayloadAttributeValueScheme{
			{
				Value: val,
			},
		},
	}, nil
}

func getConfigStatusIDByName(status string, statusType *[]StatusTypeMetadata) (string, error) {
	statuses := []string{}
	for _, statusType := range *statusType {
		statuses = append(statuses, statusType.Name)
		if statusType.Name == status {
			return statusType.ID, nil
		}
	}
	return "", fmt.Errorf("unknown status, available statuses: " + strings.Join(statuses, ","))
}

func getConfigStatusNameByID(id string, statusType *[]StatusTypeMetadata) string {
	for _, statusType := range *statusType {
		if statusType.ID == id {
			return statusType.Name
		}
	}
	return ""
}

// Schema defines the schema for the resource.
func (r *objectResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A Jira Assets object resource.",
		Attributes: map[string]schema.Attribute{
			"workspace_id": schema.StringAttribute{
				Computed:    true,
				Description: "The ID of the workspace the object belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"global_id": schema.StringAttribute{
				Computed:    true,
				Description: "The global ID of the object.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The ID of the object.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"label": schema.StringAttribute{
				Computed:    true,
				Description: "The name of the object. This value is fetched from the attribute that is currently marked as label for the object type of this object",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"object_key": schema.StringAttribute{
				Computed:    true,
				Description: "The external identifier for this object",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"type": schema.StringAttribute{
				Required: true,
			},
			"attributes": schema.MapAttribute{
				Required:    true,
				Description: "Kay value pairs of the attributes of the object",
				ElementType: types.StringType,
			},
			"created": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated": schema.StringAttribute{
				Computed: true,
			},
			"has_avatar": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"avatar_uuid": schema.StringAttribute{
				Optional:    true,
				Description: "The UUID as retrieved by uploading an avatar.",
			},
		},
	}
}

// Create creates the resource and sets the initial Terraform state.
func (r *objectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan objectResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	object_type_id := getObjectTypeByName(plan.Type.ValueString(), r.objectSchemaTypes)

	elements := make(map[string]types.String, len(plan.Attributes.Elements()))
	plan.Attributes.ElementsAs(ctx, &elements, false)

	var attributes []*models.ObjectPayloadAttributeScheme
	for attr_type, attr_value := range elements {
		v, e := returnAttributePayloadValue(attr_type, attr_value.ValueString(), object_type_id.Name, r.objectSchemaAttributes, r.configStatusType)
		if e != nil {
			tflog.Error(ctx, e.Error())
			resp.Diagnostics.AddError(
				"Error during object attributes setting",
				e.Error(),
			)
			return
		}
		attributes = append(attributes, v)
	}

	// create payload
	payload := &models.ObjectPayloadScheme{
		ObjectTypeID: object_type_id.Id,
		Attributes:   attributes,
		HasAvatar:    plan.HasAvatar.ValueBool(),
		AvatarUUID:   plan.AvatarUuid.ValueString(),
	}

	object, response, err := r.client.Object.Create(ctx, r.workspaceId, payload)
	if err != nil {
		if response != nil {
			tflog.Error(ctx, "Error creating object: %s", map[string]interface{}{
				"url":         response.Request.URL,
				"status_code": response.StatusCode,
				"headers":     response.Header,
				"body":        response.Body,
			})
		}

		resp.Diagnostics.AddError(
			"Error during object creation",
			err.Error(),
		)
		return
	}

	// Map response body to schema and populate Computed attributes
	plan.WorkspaceId = types.StringValue(object.WorkspaceId)
	plan.GlobalId = types.StringValue(object.GlobalId)
	plan.Id = types.StringValue(object.ID)
	plan.Label = types.StringValue(object.Label)
	plan.ObjectKey = types.StringValue(object.ObjectKey)
	plan.Created = types.StringValue(object.Created)
	plan.Updated = types.StringValue(object.Updated)
	plan.HasAvatar = types.BoolValue(object.HasAvatar)

	// Set state to full populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *objectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state objectResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get refreshed object from Assets API
	object, response, err := r.client.Object.Get(ctx, r.workspaceId, state.Id.ValueString())
	if err != nil {
		if response != nil {
			tflog.Error(ctx, "Error reading object: %s", map[string]interface{}{
				"url":         response.Request.URL,
				"status_code": response.StatusCode,
				"headers":     response.Header,
				"body":        response.Body,
			})
		}

		resp.Diagnostics.AddError(
			"Error during object reading",
			err.Error(),
		)
		return
	}

	// Get refreshed object attributes from Assets API
	attrs, response, err := r.client.Object.Attributes(ctx, r.workspaceId, state.Id.ValueString())
	if err != nil {
		if response != nil {
			tflog.Error(ctx, "Error reading object attributes: %s", map[string]interface{}{
				"url":         response.Request.URL,
				"status_code": response.StatusCode,
				"headers":     response.Header,
				"body":        response.Body,
			})
		}
		resp.Diagnostics.AddError(
			"Error during object attributes reading",
			err.Error(),
		)
		return
	}
	attributes := make(map[string]string)
	for _, attr := range attrs {
		// only map known attributes in the state, this is because the API return computed attributes like "key", "created",
		// and "updated". CI Class in my instance also messes up the state
		ignore_keys := append([]string{"Created", "Key", "Updated"}, r.ignoreKeys...)
		if !(slices.Contains(ignore_keys, attr.ObjectTypeAttribute.Name)) {
			attributes[attr.ObjectTypeAttribute.Name], _ = getAttributeValue(attr, r.configStatusType)
		}
	}
	mapValue, _ := types.MapValueFrom(ctx, types.StringType, attributes)
	// Overwrite items in state with refreshed values
	state.Attributes = mapValue
	state.WorkspaceId = types.StringValue(object.WorkspaceId)
	state.GlobalId = types.StringValue(object.GlobalId)
	state.Id = types.StringValue(object.ID)
	state.Label = types.StringValue(object.Label)
	state.ObjectKey = types.StringValue(object.ObjectKey)
	state.HasAvatar = types.BoolValue(object.HasAvatar)
	state.Type = types.StringValue(object.ObjectType.Name)

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *objectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan objectResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	object_type_id := getObjectTypeByName(plan.Type.ValueString(), r.objectSchemaTypes)

	// Generate API request body from plan
	// if an attribute is removed from plan, it will not be removed from the object
	// this is due to how the API only partially updates the object
	elements := make(map[string]types.String, len(plan.Attributes.Elements()))
	plan.Attributes.ElementsAs(ctx, &elements, false)
	var attributes []*models.ObjectPayloadAttributeScheme
	for attr_type, attr_value := range elements {
		v, e := returnAttributePayloadValue(attr_type, attr_value.ValueString(), object_type_id.Name, r.objectSchemaAttributes, r.configStatusType)
		if e != nil {
			tflog.Error(ctx, e.Error())
			resp.Diagnostics.AddError(
				"Error during object attributes setting",
				e.Error(),
			)
			return
		}
		attributes = append(attributes, v)
	}

	// create payload
	payload := &models.ObjectPayloadScheme{
		ObjectTypeID: object_type_id.Id,
		Attributes:   attributes,
		HasAvatar:    plan.HasAvatar.ValueBool(),
		AvatarUUID:   plan.AvatarUuid.ValueString(),
	}

	// update object
	tflog.Info(ctx, "Updating object.", map[string]interface{}{
		"Id": plan.Id.ValueString(),
	})
	object, response, err := r.client.Object.Update(ctx, r.workspaceId, plan.Id.ValueString(), payload)
	if err != nil {
		if response != nil {
			tflog.Error(ctx, "Error updating object: %s", map[string]interface{}{
				"url":         response.Request.URL,
				"status_code": response.StatusCode,
				"headers":     response.Header,
				"body":        response.Body,
			})
		}

		resp.Diagnostics.AddError(
			"Error during object update",
			err.Error(),
		)
		return
	}

	// Update resource state with updated object and attributes
	plan.WorkspaceId = types.StringValue(object.WorkspaceId)
	plan.GlobalId = types.StringValue(object.GlobalId)
	plan.Id = types.StringValue(object.ID)
	plan.Label = types.StringValue(object.Label)
	plan.ObjectKey = types.StringValue(object.ObjectKey)
	plan.Created = types.StringValue(object.Created)
	plan.Updated = types.StringValue(object.Updated)
	plan.HasAvatar = types.BoolValue(object.HasAvatar)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *objectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state objectResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete existing object
	response, err := r.client.Object.Delete(ctx, r.workspaceId, state.Id.ValueString())
	if err != nil {
		if response != nil {
			tflog.Error(ctx, "Error deleting object: %s", map[string]interface{}{
				"url":         response.Request.URL,
				"status_code": response.StatusCode,
				"headers":     response.Header,
				"body":        response.Body,
			})
		}

		resp.Diagnostics.AddError(
			"Error during object deletion",
			err.Error(),
		)
		return
	}
}

func (r *objectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Configure configures the resource with the given configuration.
func (r *objectResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	providerClient, ok := req.ProviderData.(JiraAssetsProviderClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *hashicups.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = providerClient.client
	r.workspaceId = providerClient.workspaceId
	r.objectschemaId = providerClient.objectschemaId
	r.ignoreKeys = providerClient.ignoreKeys
	r.objectSchemaTypes = providerClient.objectSchemaTypes
	r.objectSchemaAttributes = providerClient.objectSchemaAttributes
	r.configStatusType = providerClient.configStatusType
}
