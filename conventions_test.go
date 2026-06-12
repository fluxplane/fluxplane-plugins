package registry_test

// Cross-plugin convention gates (fluxplane-plugins#12). Each rule carries a
// shrinking allowlist seeded with the deviations that existed when the rule
// landed: fixing a plugin deletes its entries, a stale entry fails the test,
// and an empty allowlist means the convention is enforced forever.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// walkGoFiles parses every non-test .go file under the repo root and hands the
// AST to visit together with the top-level plugin directory name.
func walkGoFiles(t *testing.T, visit func(plugin, path string, file *ast.File)) {
	t.Helper()
	err := filepath.WalkDir(".", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "vendor":
				return filepath.SkipDir
			default:
				return nil
			}
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		plugin := filepath.ToSlash(filepath.Dir(path))
		if plugin == "." { // repo-root file, not a plugin
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		visit(plugin, path, file)
		return nil
	})
	if err != nil {
		t.Fatalf("walk plugin repo: %v", err)
	}
}

// assertAllowlisted compares found violations against the rule's allowlist:
// new violations fail, and so do stale allowlist entries — the list only ever
// shrinks.
func assertAllowlisted(t *testing.T, rule string, found []string, allowed map[string]bool) {
	t.Helper()
	sort.Strings(found)
	var fresh []string
	seen := map[string]bool{}
	for _, violation := range found {
		seen[violation] = true
		if !allowed[violation] {
			fresh = append(fresh, violation)
		}
	}
	var stale []string
	for entry := range allowed {
		if !seen[entry] {
			stale = append(stale, entry)
		}
	}
	sort.Strings(stale)
	if len(fresh) > 0 {
		t.Errorf("%s: new violations (fix them, do not extend the allowlist):\n  %s", rule, strings.Join(fresh, "\n  "))
	}
	if len(stale) > 0 {
		t.Errorf("%s: stale allowlist entries (fixed — delete them):\n  %s", rule, strings.Join(stale, "\n  "))
	}
}

// TestProbeOperationNaming: a plugin's connectivity probe is `<plugin>.test`,
// nothing else. Any operation-name literal ending in `.test` must have exactly
// two segments.
func TestProbeOperationNaming(t *testing.T) {
	allowed := map[string]bool{
		`gitlab: "gitlab.auth.test"`:            true,
		`kubernetes: "kubernetes.cluster.test"`: true,
		`slack: "slack.auth.test"`:              true,
	}
	var found []string
	walkGoFiles(t, func(plugin, path string, file *ast.File) {
		if filepath.Base(path) != "manifest.go" {
			return
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			value := strings.Trim(lit.Value, `"`)
			if !strings.HasSuffix(value, ".test") {
				return true
			}
			if strings.Count(value, ".") != 1 {
				found = append(found, fmt.Sprintf("%s: %q", plugin, value))
			}
			return true
		})
	})
	assertAllowlisted(t, "probe operations must be named <plugin>.test", found, allowed)
}

// TestResultSlicesAreNeverOmitted: collection fields on result structs are
// always present in output — `[]`, never `null`, never a missing key. An
// omitempty on a slice field of a *Result struct silently drops the key on
// empty results and crashes naive consumers.
func TestResultSlicesAreNeverOmitted(t *testing.T) {
	allowed := seededOmitemptyAllowlist()
	var found []string
	walkGoFiles(t, func(plugin, path string, file *ast.File) {
		ast.Inspect(file, func(n ast.Node) bool {
			spec, ok := n.(*ast.TypeSpec)
			if !ok || !strings.HasSuffix(spec.Name.Name, "Result") {
				return true
			}
			structType, ok := spec.Type.(*ast.StructType)
			if !ok {
				return true
			}
			for _, field := range structType.Fields.List {
				arrayType, isSlice := field.Type.(*ast.ArrayType)
				if !isSlice || field.Tag == nil {
					continue
				}
				// []byte marshals to a base64 string, not a JSON collection.
				if ident, ok := arrayType.Elt.(*ast.Ident); ok && (ident.Name == "byte" || ident.Name == "uint8") {
					continue
				}
				tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
				jsonTag := tag.Get("json")
				if !strings.Contains(jsonTag, ",omitempty") {
					continue
				}
				for _, name := range field.Names {
					found = append(found, fmt.Sprintf("%s: %s.%s", plugin, spec.Name.Name, name.Name))
				}
			}
			return true
		})
	})
	assertAllowlisted(t, "result-struct slice fields must not be omitempty", found, allowed)
}

var timeRangeDescription = regexp.MustCompile(`(?i)time|rfc3339|timestamp`)

// TestTimeRangeFieldNaming: query time ranges are `since`/`until` everywhere.
// An input field json-named start/end whose description marks it as a time is
// a vocabulary fork that costs agents an invalid-input round trip.
func TestTimeRangeFieldNaming(t *testing.T) {
	allowed := map[string]bool{
		"grafana: PrometheusRangeInput.End":   true,
		"grafana: PrometheusRangeInput.Start": true,
		"grafana: TempoSearchInput.End":       true,
		"grafana: TempoSearchInput.Start":     true,
		"prometheus: QueryRangeInput.End":     true,
		"prometheus: QueryRangeInput.Start":   true,
		"prometheus: SeriesInput.End":         true,
		"prometheus: SeriesInput.Start":       true,
	}
	var found []string
	walkGoFiles(t, func(plugin, path string, file *ast.File) {
		ast.Inspect(file, func(n ast.Node) bool {
			spec, ok := n.(*ast.TypeSpec)
			if !ok || !strings.HasSuffix(spec.Name.Name, "Input") {
				return true
			}
			structType, ok := spec.Type.(*ast.StructType)
			if !ok {
				return true
			}
			for _, field := range structType.Fields.List {
				if field.Tag == nil {
					continue
				}
				tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
				jsonName, _, _ := strings.Cut(tag.Get("json"), ",")
				if jsonName != "start" && jsonName != "end" {
					continue
				}
				if !timeRangeDescription.MatchString(tag.Get("jsonschema")) {
					continue
				}
				for _, name := range field.Names {
					found = append(found, fmt.Sprintf("%s: %s.%s", plugin, spec.Name.Name, name.Name))
				}
			}
			return true
		})
	})
	assertAllowlisted(t, "time-range inputs must be named since/until", found, allowed)
}

// seededOmitemptyAllowlist is the omitempty-slice state when the rule landed.
// Entries are deleted as plugins fix them — never added.
func seededOmitemptyAllowlist() map[string]bool {
	return map[string]bool{
		"asterisk: CommandResult.Lines":              true,
		"aws: CloudWatchMetricsResult.Datapoints":    true,
		"aws: EC2InstancesResult.Instances":          true,
		"aws: EKSClustersResult.Clusters":            true,
		"aws: LogsGroupsResult.Groups":               true,
		"aws: LogsQueryResult.Columns":               true,
		"aws: LogsQueryResult.Rows":                  true,
		"aws: LogsTailResult.Events":                 true,
		"aws: RDSInstancesResult.Clusters":           true,
		"aws: RDSInstancesResult.Instances":          true,
		"aws: S3BucketsResult.Buckets":               true,
		"aws: S3ObjectsResult.Objects":               true,
		"docker: ContainerCopyResult.Files":          true,
		"docker: ContainerCreateResult.Warnings":     true,
		"docker: ContainerTopResult.Processes":       true,
		"docker: ContainerTopResult.Titles":          true,
		"docker: ImageBuildResult.Events":            true,
		"docker: ImageBuildResult.Tags":              true,
		"docker: ImagePruneResult.Deleted":           true,
		"docker: ImagePruneResult.Untagged":          true,
		"docker: ImagePullResult.Events":             true,
		"docker: ImagePushResult.Events":             true,
		"docker: ImageRemoveResult.Deleted":          true,
		"docker: ImageRemoveResult.Untagged":         true,
		"docker: PruneResult.Deleted":                true,
		"docker: SystemDFResult.Containers":          true,
		"docker: SystemDFResult.Images":              true,
		"docker: SystemDFResult.Volumes":             true,
		"gitlab: BlobSearchResult.Matches":           true,
		"gitlab: CompareResult.Commits":              true,
		"gitlab: CompareResult.Files":                true,
		"gitlab: MRChangesResult.Files":              true,
		"gitlab: MRDiffLinesResult.Lines":            true,
		"gitlab: MRDiscussionCreateResult.Lines":     true,
		"gitlab: MRDiscussionListResult.Discussions": true,
		"gitlab: RepositoryTreeResult.Entries":       true,
		"gitlab: datasourceBatchGetResult.Errors":    true,
		"grafana: DashboardGetResult.Panels":         true,
		"grafana: DashboardGetResult.Queries":        true,
		"grafana: PromQueryResult.Samples":           true,
		"grafana: PromQueryResult.Series":            true,
		"grafana: TempoTraceResult.Services":         true,
		"homer: CallAnalyzeResult.CorrelationValues": true,
		"homer: CallAnalyzeResult.Events":            true,
		"kubernetes: PortForwardResult.Command":      true,
		"prometheus: QueryResult.Samples":            true,
		"prometheus: QueryResult.Series":             true,
		"slack: AuthTestResult.Tokens":               true,
		"slack: BookmarkListResult.Bookmarks":        true,
		"slack: ChannelListResult.Channels":          true,
		"slack: EmojiListResult.Emojis":              true,
		"slack: FileListResult.Files":                true,
		"slack: InfoResult.Tokens":                   true,
		"slack: MentionsResult.Mentions":             true,
		"slack: MentionsResult.Tickets":              true,
		"slack: MessageListResult.Messages":          true,
		"slack: SearchResult.Messages":               true,
		"slack: SearchResult.Tickets":                true,
		"slack: ThreadResult.Messages":               true,
		"slack: UnreadsResult.Channels":              true,
		"slack: UserListResult.Users":                true,
	}
}
