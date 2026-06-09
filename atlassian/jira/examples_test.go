package jira

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestOperationExamplesDeclared verifies the conditional/non-obvious write ops
// carry a runnable JSON Schema example, so `operation describe` shows a usable
// invocation and local --dry-run treats them as one-of inputs.
func TestOperationExamplesDeclared(t *testing.T) {
	specs := map[string]json.RawMessage{
		OperationTransitionRun: transitionRunSpec().Input,
		OperationAttachmentAdd: attachmentAddSpec().Input,
		OperationIssueCreate:   issueCreateSpec().Input,
		OperationIssueEdit:     issueEditSpec().Input,
		OperationCommentAdd:    commentAddSpec().Input,
	}
	wants := map[string][]string{
		OperationTransitionRun: {"target_status", "auto_transition"},
		OperationAttachmentAdd: {"blob_ref"},
		OperationIssueCreate:   {"project_key", "description_markdown"},
		OperationIssueEdit:     {"parent_key"},
		OperationCommentAdd:    {"body_markdown"},
	}

	for name, input := range specs {
		var schema struct {
			Examples []map[string]any `json:"examples"`
		}
		if err := json.Unmarshal(input, &schema); err != nil {
			t.Fatalf("%s: unmarshal input schema: %v", name, err)
		}
		if len(schema.Examples) == 0 {
			t.Fatalf("%s: expected a declared example", name)
		}
		raw, _ := json.Marshal(schema.Examples[0])
		for _, key := range wants[name] {
			if !strings.Contains(string(raw), `"`+key+`"`) {
				t.Fatalf("%s example %s missing %q", name, raw, key)
			}
		}
	}
}
