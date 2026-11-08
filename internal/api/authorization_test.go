package api

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"

	"github.com/stashapp/stash/pkg/user"
)

var (
	admin  = user.User{ID: 1, Username: "admin", Role: user.RoleAdmin}
	editor = user.User{ID: 2, Username: "editor", Role: user.RoleEditor}
	viewer = user.User{ID: 3, Username: "viewer", Role: user.RoleViewer}
)

func loadSchema(t *testing.T) *ast.Schema {
	t.Helper()

	var sources []*ast.Source
	for _, pattern := range []string{"../../graphql/schema/*.graphql", "../../graphql/schema/types/*.graphql"} {
		files, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			sources = append(sources, &ast.Source{Name: f, Input: string(b)})
		}
	}

	schema, err := gqlparser.LoadSchema(sources...)
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

// TestAllRootFieldsClassified ensures that every root field of the schema
// has an explicit authorization rule, and that no rule refers to a field
// that does not exist.
func TestAllRootFieldsClassified(t *testing.T) {
	schema := loadSchema(t)

	for object, def := range map[string]*ast.Definition{
		rootQuery:        schema.Query,
		rootMutation:     schema.Mutation,
		rootSubscription: schema.Subscription,
	} {
		objRules := rootFieldRules[object]
		inSchema := make(map[string]bool)
		for _, f := range def.Fields {
			if f.Name == "__schema" || f.Name == "__type" || f.Name == "__typename" {
				continue
			}
			inSchema[f.Name] = true
			if _, ok := objRules[f.Name]; !ok {
				t.Errorf("%s.%s has no authorization rule", object, f.Name)
			}
		}
		for name := range objRules {
			if !inSchema[name] {
				t.Errorf("authorization rule for unknown field %s.%s", object, name)
			}
		}
	}
}

func TestAuthorizeRootField(t *testing.T) {
	tests := []struct {
		name    string
		u       user.User
		object  string
		field   string
		args    map[string]interface{}
		allowed bool
	}{
		{"viewer find scenes", viewer, rootQuery, "findScenes", nil, true},
		{"viewer configuration", viewer, rootQuery, "configuration", nil, true},
		{"viewer logs", viewer, rootQuery, "logs", nil, false},
		{"viewer directory", viewer, rootQuery, "directory", nil, false},
		{"editor scrape", editor, rootQuery, "scrapeSceneURL", nil, true},
		{"viewer scrape", viewer, rootQuery, "scrapeSceneURL", nil, false},
		{"unknown query is admin only", editor, rootQuery, "someNewQuery", nil, false},
		{"admin unknown query", admin, rootQuery, "someNewQuery", nil, true},

		{"viewer add play", viewer, rootMutation, "sceneAddPlay", nil, true},
		{"viewer increment o", viewer, rootMutation, "sceneIncrementO", nil, true},
		{"viewer configure general", viewer, rootMutation, "configureGeneral", nil, false},
		{"editor configure general", editor, rootMutation, "configureGeneral", nil, false},
		{"admin configure general", admin, rootMutation, "configureGeneral", nil, true},
		{"editor scan", editor, rootMutation, "metadataScan", nil, false},
		{"viewer exec sql", viewer, rootMutation, "execSQL", nil, false},
		{"editor exec sql", editor, rootMutation, "execSQL", nil, false},
		{"editor delete files", editor, rootMutation, "deleteFiles", nil, false},
		{"editor tag update", editor, rootMutation, "tagUpdate", nil, true},
		{"viewer tag update", viewer, rootMutation, "tagUpdate", nil, false},
		{"unknown mutation is admin only", editor, rootMutation, "someNewMutation", nil, false},

		{"viewer rating update", viewer, rootMutation, "sceneUpdate",
			map[string]interface{}{"input": map[string]interface{}{"id": "1", "rating100": 80}}, true},
		{"viewer title update", viewer, rootMutation, "sceneUpdate",
			map[string]interface{}{"input": map[string]interface{}{"id": "1", "title": "x"}}, false},
		{"viewer bulk rating update", viewer, rootMutation, "bulkSceneUpdate",
			map[string]interface{}{"input": map[string]interface{}{"ids": []interface{}{"1"}, "rating100": 80}}, true},
		{"viewer scenesUpdate list", viewer, rootMutation, "scenesUpdate",
			map[string]interface{}{"input": []interface{}{
				map[string]interface{}{"id": "1", "rating100": 80},
				map[string]interface{}{"id": "2", "organized": true},
			}}, false},
		{"editor title update", editor, rootMutation, "sceneUpdate",
			map[string]interface{}{"input": map[string]interface{}{"id": "1", "title": "x"}}, true},

		{"editor destroy scene", editor, rootMutation, "sceneDestroy",
			map[string]interface{}{"input": map[string]interface{}{"id": "1"}}, true},
		{"editor destroy scene with file", editor, rootMutation, "sceneDestroy",
			map[string]interface{}{"input": map[string]interface{}{"id": "1", "delete_file": true}}, false},
		{"editor destroy gallery file entry", editor, rootMutation, "galleryDestroy",
			map[string]interface{}{"input": map[string]interface{}{"ids": []interface{}{"1"}, "destroy_file_entry": true}}, false},
		{"admin destroy scene with file", admin, rootMutation, "sceneDestroy",
			map[string]interface{}{"input": map[string]interface{}{"id": "1", "delete_file": true}}, true},
		{"viewer destroy scene", viewer, rootMutation, "sceneDestroy",
			map[string]interface{}{"input": map[string]interface{}{"id": "1"}}, false},

		{"viewer jobs", viewer, rootSubscription, "jobsSubscribe", nil, true},
		{"viewer logging", viewer, rootSubscription, "loggingSubscribe", nil, false},

		{"non-root object", viewer, "Scene", "files", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := authorizeRootField(tt.u, tt.object, tt.field, tt.args)
			if tt.allowed && err != nil {
				t.Errorf("expected allowed, got %v", err)
			}
			if !tt.allowed {
				if err == nil {
					t.Error("expected forbidden")
				} else if !errors.Is(err, ErrForbidden) {
					t.Errorf("expected ErrForbidden, got %v", err)
				}
			}
		})
	}
}
