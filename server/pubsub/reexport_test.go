package pubsub_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"
)

// The re-exports in server/pubsub/api.go and device/pubsub/api.go are written
// by hand. These tests fail when internal/ably gains an exported name that an
// entry point neither re-exports nor lists below, so that leaving a name out
// is always a recorded decision rather than an oversight.

// notReExported lists the internal/ably names that neither entry point
// re-exports, with the reason.
var notReExported = map[string]string{
	"Realtime":                      "exposed as RealtimeClient (server) and Client (device)",
	"NewRealtime":                   "wrapped by NewRealtimeClient (server) and NewClient (device) to declare the side agent",
	"WithDial":                      "its signature names an unexported type, so no caller can supply a usable argument",
	"ErrChannelReleaseInvalidState": "intentionally internal; callers compare ErrorInfo.Code against 90011",
}

// deviceOmits lists the names that only an HTTP client can reach. A device
// client has no HTTP client, so device/pubsub leaves them out.
var deviceOmits = []string{
	"ChannelDetails", "ChannelMetrics", "ChannelOccupancy", "ChannelStatus",
	"GetPresenceOption", "GetPresenceWithClientID", "GetPresenceWithConnectionID", "GetPresenceWithLimit",
	"HTTPChannel", "HTTPChannels", "HTTPClient", "HTTPPaginatedItems", "HTTPPaginatedResponse",
	"HTTPPresence", "HTTPRequest", "NewHTTPClient",
	"PresenceHistoryOption", "PresenceHistoryWithDirection", "PresenceHistoryWithEnd",
	"PresenceHistoryWithLimit", "PresenceHistoryWithStart",
	"PresencePaginatedItems", "PresencePaginatedResult", "PresenceRequest",
	"PublishMultipleOption", "PublishMultipleWithParams", "PublishWithConnectionKey", "PublishWithParams",
	"RequestOption", "RequestWithBody", "RequestWithHeaders", "RequestWithParams",
}

func TestServerReExportsInternalAPI(t *testing.T) {
	missing := missingFrom(t, "../../server/pubsub", nil)
	if len(missing) > 0 {
		t.Errorf("internal/ably exports names that server/pubsub/api.go does not re-export: %s\n"+
			"Add them to server/pubsub/api.go, or to notReExported with a reason.", strings.Join(missing, ", "))
	}
}

func TestDeviceReExportsInternalAPI(t *testing.T) {
	omitted := map[string]bool{}
	for _, name := range deviceOmits {
		omitted[name] = true
	}
	missing := missingFrom(t, "../../device/pubsub", omitted)
	if len(missing) > 0 {
		t.Errorf("internal/ably exports names that device/pubsub/api.go does not re-export: %s\n"+
			"Add them to device/pubsub/api.go, or to deviceOmits if only an HTTP client can reach them.", strings.Join(missing, ", "))
	}

	device := exportedNames(t, "../../device/pubsub", "", false)
	for _, name := range deviceOmits {
		if device[name] {
			t.Errorf("device/pubsub re-exports %s, which deviceOmits says only an HTTP client can reach", name)
		}
	}
}

// missingFrom returns the internal/ably names, plus the LiveObjects names under
// their Objects prefix, that the package in dir does not declare and that are
// neither in notReExported nor in also.
func missingFrom(t *testing.T, dir string, also map[string]bool) []string {
	t.Helper()
	want := exportedNames(t, "../../internal/ably", "", false)
	for name := range exportedNames(t, "../../internal/ably/objects", "Objects", true) {
		want[name] = true
	}
	have := exportedNames(t, dir, "", false)

	var missing []string
	for name := range want {
		if !have[name] && notReExported[name] == "" && !also[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return missing
}

// exportedNames returns the exported package-level names declared in the
// non-test files of the package in dir. The prefix and the dropping of
// underscores give a LiveObjects name its re-exported spelling, so that
// objects.Operation_MapSet is looked for as ObjectsOperationMapSet.
func exportedNames(t *testing.T, dir, prefix string, dropUnderscores bool) map[string]bool {
	t.Helper()
	pkgs, err := parser.ParseDir(token.NewFileSet(), dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", dir, err)
	}
	names := map[string]bool{}
	add := func(name string) {
		if !ast.IsExported(name) {
			return
		}
		if dropUnderscores {
			name = strings.ReplaceAll(name, "_", "")
		}
		names[prefix+name] = true
	}
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				switch decl := decl.(type) {
				case *ast.FuncDecl:
					if decl.Recv == nil {
						add(decl.Name.Name)
					}
				case *ast.GenDecl:
					for _, spec := range decl.Specs {
						switch spec := spec.(type) {
						case *ast.TypeSpec:
							add(spec.Name.Name)
						case *ast.ValueSpec:
							for _, name := range spec.Names {
								add(name.Name)
							}
						}
					}
				}
			}
		}
	}
	return names
}
