/*
*TBD*

# Example Usage

```hcl
resource "random_uuid" "workflow_node_base_uuid" {}

	resource "awx_workflow_job_template_node" "default" {
	  workflow_job_template_id = awx_workflow_job_template.default.id
	  unified_job_template_id  = awx_job_template.baseconfig.id
	  inventory_id             = awx_inventory.default.id
	  identifier               = random_uuid.workflow_node_base_uuid.result
	}

```
*/
package awx

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	awx "gitlab.iwd.re/dev-team-ops/goawx/client"
)

func resourceWorkflowJobTemplateNode() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceWorkflowJobTemplateNodeCreate,
		ReadContext:   resourceWorkflowJobTemplateNodeRead,
		UpdateContext: resourceWorkflowJobTemplateNodeUpdate,
		DeleteContext: resourceWorkflowJobTemplateNodeDelete,

		Schema: map[string]*schema.Schema{

			"extra_data": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "",
				StateFunc:   normalizeJsonYaml,
			},
			"inventory_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Inventory applied as a prompt, assuming job template prompts for inventory.",
			},
			"scm_branch": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			"job_type": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "run",
			},
			"job_tags": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"skip_tags": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"limit": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"diff_mode": {
				Type:     schema.TypeBool,
				Optional: true,
			},
			"verbosity": {
				Type:     schema.TypeInt,
				Optional: true,
				Default:  0,
			},
			"workflow_job_template_id": {
				Type:     schema.TypeInt,
				Required: true,
			},
			"unified_job_template_id": {
				Type:     schema.TypeInt,
				Required: true,
			},
			"success_nodes": &schema.Schema{
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeInt,
				},
				Optional: true,
				Description: "List of node IDs to start when this node has completed successfully.",
			},
			"failure_nodes": &schema.Schema{
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeInt,
				},
				Optional: true,
				Description: "List of node IDs to start when this node has failed.",
			},
			"always_nodes": &schema.Schema{
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeInt,
				},
				Optional: true,
				Description: "List of node IDs to start when this node has finished.",
			},

			"all_parents_must_converge": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			"identifier": {
				Type:     schema.TypeString,
				Required: true,
			},
		},
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		//
		//Timeouts: &schema.ResourceTimeout{
		//	Create: schema.DefaultTimeout(1 * time.Minute),
		//	Update: schema.DefaultTimeout(1 * time.Minute),
		//	Delete: schema.DefaultTimeout(1 * time.Minute),
		//},
	}
}

// Helper function to sort and deduplicate an array of integers
func normalizeIntArray(arr []int) []int {
	if len(arr) == 0 {
		return arr
	}
	
	// Convert to map to remove duplicates
	uniqueMap := make(map[int]bool)
	for _, num := range arr {
		uniqueMap[num] = true
	}
	
	// Convert back to slice
	result := make([]int, 0, len(uniqueMap))
	for num := range uniqueMap {
		result = append(result, num)
	}
	
	// Sort the slice
	sort.Ints(result)
	return result
}

// Helper function to convert schema.TypeList to []int and normalize it
func getNodeList(d *schema.ResourceData, field string) []int {
	var result []int
	if nodes, ok := d.GetOk(field); ok {
		nodeList := nodes.([]interface{})
		for _, node := range nodeList {
			result = append(result, node.(int))
		}
	}
	return normalizeIntArray(result)
}


func resourceWorkflowJobTemplateNodeCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	client := m.(*awx.AWX)
	awxService := client.WorkflowJobTemplateNodeService

	// Create params map
	params := map[string]interface{}{
		"extra_data":                d.Get("extra_data").(string),
		"workflow_job_template":     d.Get("workflow_job_template_id").(int),
		"unified_job_template":      d.Get("unified_job_template_id").(int),
		"all_parents_must_converge": d.Get("all_parents_must_converge").(bool),
		"identifier":                d.Get("identifier").(string),
	}
	
	if d.Get("job_type").(string) == "run" {
		params["job_type"] = d.Get("job_type").(string)
    params["verbosity"] = d.Get("verbosity").(int)
		params["limit"] = d.Get("limit").(string)
		params["scm_branch"] = d.Get("scm_branch").(string)
		params["job_tags"] = d.Get("job_tags").(string)
		params["skip_tags"] = d.Get("skip_tags").(string)
		params["diff_mode"] = d.Get("diff_mode").(bool)
	}

	// Only add inventory if it's set
	if v, ok := d.GetOk("inventory_id"); ok {
		params["inventory"] = v.(int)
	}

	result, err := awxService.CreateWorkflowJobTemplateNode(params, map[string]string{})
	if err != nil {
		log.Printf("Fail to Create Template %v", err)
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "Unable to create WorkflowJobTemplateNode",
			Detail:   fmt.Sprintf("WorkflowJobTemplateNode with JobTemplateID %d and WorkflowID: %d failed to create %s", d.Get("unified_job_template_id").(int), d.Get("workflow_job_template_id").(int), err.Error()),
		})
		return diags
	}

	d.SetId(strconv.Itoa(result.ID))

	// Handle relationships after node creation
	if err := handleNodeRelationships(client.WorkflowJobTemplateNodeService, result.ID, d); err != nil {
		return diag.FromErr(err)
	}

	return resourceWorkflowJobTemplateNodeRead(ctx, d, m)
}

func resourceWorkflowJobTemplateNodeUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	client := m.(*awx.AWX)
	awxService := client.WorkflowJobTemplateNodeService
	id, diags := convertStateIDToNummeric("Update WorkflowJobTemplateNode", d)
	if diags.HasError() {
		return diags
	}

	params := make(map[string]string)
	_, err := awxService.GetWorkflowJobTemplateNodeByID(id, params)
	if err != nil {
		return buildDiagNotFoundFail("workflow job template node", id, err)
	}

	// Create update params map
	updateParams := map[string]interface{}{
		"extra_data":                d.Get("extra_data").(string),
		"workflow_job_template":     d.Get("workflow_job_template_id").(int),
		"unified_job_template":      d.Get("unified_job_template_id").(int),
		"all_parents_must_converge": d.Get("all_parents_must_converge").(bool),
		"identifier":                d.Get("identifier").(string),
	}

	if d.Get("job_type").(string) == "run" {
		updateParams["job_type"] = d.Get("job_type").(string)
    updateParams["verbosity"] = d.Get("verbosity").(int)
		updateParams["limit"] = d.Get("limit").(string)
		updateParams["scm_branch"] = d.Get("scm_branch").(string)
		updateParams["job_tags"] = d.Get("job_tags").(string)
		updateParams["skip_tags"] = d.Get("skip_tags").(string)
		updateParams["diff_mode"] = d.Get("diff_mode").(bool)
	}

	// Only add inventory if it's set
	if v, ok := d.GetOk("inventory_id"); ok {
		updateParams["inventory"] = v.(int)
	}

	_, err = awxService.UpdateWorkflowJobTemplateNode(id, updateParams, map[string]string{})
	if err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "Unable to update WorkflowJobTemplateNode",
			Detail:   fmt.Sprintf("WorkflowJobTemplateNode with name %s in the project id %d failed to update %s", d.Get("name").(string), d.Get("project_id").(int), err.Error()),
		})
		return diags
	}

	// Handle relationship updates
	if err := handleNodeRelationships(client.WorkflowJobTemplateNodeService, id, d); err != nil {
		return diag.FromErr(err)
	}

	return resourceWorkflowJobTemplateNodeRead(ctx, d, m)
}

func resourceWorkflowJobTemplateNodeRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	client := m.(*awx.AWX)
	awxService := client.WorkflowJobTemplateNodeService
	id, diags := convertStateIDToNummeric("Read WorkflowJobTemplateNode", d)
	if diags.HasError() {
		return diags
	}

	res, err := awxService.GetWorkflowJobTemplateNodeByID(id, make(map[string]string))
	if err != nil {
		return buildDiagNotFoundFail("workflow job template node", id, err)

	}
	
	// Get related nodes
	successNodes, err := awxService.GetNodeRelationships(id, "success_nodes")
	if err != nil {
		return diag.FromErr(err)
	}
	failureNodes, err := awxService.GetNodeRelationships(id, "failure_nodes")
	if err != nil {
		return diag.FromErr(err)
	}
	alwaysNodes, err := awxService.GetNodeRelationships(id, "always_nodes")
	if err != nil {
		return diag.FromErr(err)
	}

	// Convert node lists to []int for storage
	successIDs := make([]int, len(successNodes))
	for i, node := range successNodes {
		successIDs[i] = node.ID
	}
	successIDs = normalizeIntArray(successIDs)

	failureIDs := make([]int, len(failureNodes))
	for i, node := range failureNodes {
		failureIDs[i] = node.ID
	}
	failureIDs = normalizeIntArray(failureIDs)

	alwaysIDs := make([]int, len(alwaysNodes))
	for i, node := range alwaysNodes {
		alwaysIDs[i] = node.ID
	}
	alwaysIDs = normalizeIntArray(alwaysIDs)

	d = setWorkflowJobTemplateNodeResourceData(d, res)
	d.Set("success_nodes", successIDs)
	d.Set("failure_nodes", failureIDs)
	d.Set("always_nodes", alwaysIDs)

	return nil
}

func resourceWorkflowJobTemplateNodeDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*awx.AWX)
	awxService := client.WorkflowJobTemplateNodeService
	id, diags := convertStateIDToNummeric(diagElementHostTitle, d)
	if diags.HasError() {
		return diags
	}

	if _, err := awxService.DeleteWorkflowJobTemplateNode(id); err != nil {
		return buildDiagDeleteFail(
			diagElementHostTitle,
			fmt.Sprintf("id %v, got %s ",
				id, err.Error()))
	}
	d.SetId("")
	return nil
}

func setWorkflowJobTemplateNodeResourceData(d *schema.ResourceData, r *awx.WorkflowJobTemplateNode) *schema.ResourceData {

	d.Set("extra_data", normalizeJsonYaml(r.ExtraData))
	d.Set("inventory_id", r.Inventory)
	d.Set("scm_branch", r.ScmBranch)
	d.Set("job_type", r.JobType)
	d.Set("job_tags", r.JobTags)
	d.Set("skip_tags", r.SkipTags)
	d.Set("limit", r.Limit)
	d.Set("diff_mode", r.DiffMode)
	d.Set("verbosity", r.Verbosity)
	d.Set("workflow_job_template_id", r.WorkflowJobTemplate)
	d.Set("unified_job_template_id", r.UnifiedJobTemplate)
	d.Set("all_parents_must_converge", r.AllParentsMustConverge)
	d.Set("identifier", r.Identifier)

	d.SetId(strconv.Itoa(r.ID))
	return d
}

func handleNodeRelationships(awxService *awx.WorkflowJobTemplateNodeService, nodeID int, d *schema.ResourceData) error {
	relationshipEndpoints := []string{"success_nodes", "failure_nodes", "always_nodes"}

	for _, endpoint := range relationshipEndpoints {
		// Get existing relationships
		existing, err := awxService.GetNodeRelationships(nodeID, endpoint)
		if err != nil {
			return fmt.Errorf("failed to get existing %s relationships: %v", endpoint, err)
		}

		// Convert and normalize existing relationships
		existingIDs := make([]int, len(existing))
		for i, node := range existing {
			existingIDs[i] = node.ID
		}
		existingIDs = normalizeIntArray(existingIDs)

		// Get and normalize desired relationships from Terraform config
		desiredNodes := getNodeList(d, endpoint)

		// Skip if the normalized arrays are identical
		if len(existingIDs) == len(desiredNodes) {
			different := false
			for i := range existingIDs {
				if existingIDs[i] != desiredNodes[i] {
					different = true
					break
				}
			}
			if !different {
				continue
			}
		}

		// Convert to maps for comparison
		existingMap := make(map[int]bool)
		for _, id := range existingIDs {
			existingMap[id] = true
		}

		desiredMap := make(map[int]bool)
		for _, id := range desiredNodes {
			desiredMap[id] = true
		}

		// Remove relationships that are no longer desired
		for existingID := range existingMap {
			if !desiredMap[existingID] {
				if err := awxService.DisassociateNodeRelationship(nodeID, existingID, endpoint); err != nil {
					return fmt.Errorf("failed to remove %s relationship: %v", endpoint, err)
				}
			}
		}

		// Add new desired relationships
		for desiredID := range desiredMap {
			if !existingMap[desiredID] {
				if err := awxService.AssociateNodeRelationship(nodeID, desiredID, endpoint); err != nil {
					return fmt.Errorf("failed to create %s relationship: %v", endpoint, err)
				}
			}
		}
	}

	return nil
}
