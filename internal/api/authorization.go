package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/99designs/gqlgen/graphql"

	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/user"
)

// ErrForbidden is returned when the current user lacks the permissions for
// an operation.
var ErrForbidden = errors.New("forbidden")

// requireRole returns an error if the current user does not have at least the
// provided role.
func requireRole(ctx context.Context, role user.Role) error {
	u := user.Current(ctx)
	if !u.Role.AtLeast(role) {
		return fmt.Errorf("%w: requires role %s", ErrForbidden, role)
	}
	return nil
}

const (
	rootQuery        = "Query"
	rootMutation     = "Mutation"
	rootSubscription = "Subscription"
)

// fieldRule describes the permissions required for a root field.
type fieldRule struct {
	// Role is the minimum role required to use the field at all.
	Role user.Role
	// Check, if set, performs additional checks against the raw arguments.
	// It is only called if the user has at least Role.
	Check func(u user.User, args map[string]interface{}) error
}

// rootFieldRules maps object name -> field name -> rule. Fields that are not
// listed require the admin role, so that new operations are denied by default.
var rootFieldRules = map[string]map[string]fieldRule{
	rootQuery:        queryRules,
	rootMutation:     mutationRules,
	rootSubscription: subscriptionRules,
}

func rules(role user.Role, fields ...string) map[string]fieldRule {
	ret := make(map[string]fieldRule, len(fields))
	for _, f := range fields {
		ret[f] = fieldRule{Role: role}
	}
	return ret
}

func merge(maps ...map[string]fieldRule) map[string]fieldRule {
	ret := make(map[string]fieldRule)
	for _, m := range maps {
		for k, v := range m {
			if _, exists := ret[k]; exists {
				panic(fmt.Sprintf("duplicate authorization rule for %s", k))
			}
			ret[k] = v
		}
	}
	return ret
}

var queryRules = merge(
	rules(user.RoleViewer,
		// browsing
		"findSavedFilter", "findSavedFilters", "findDefaultFilter",
		"findFile", "findFiles", "findFolder", "findFolders",
		"findScene", "findSceneByHash", "findScenes", "sceneStreams",
		"findSceneMarkers", "findImage", "findImages",
		"findPerformer", "findPerformers", "findStudio", "findStudios",
		"findMovie", "findMovies", "findGroup", "findGroups",
		"findGallery", "findGalleries", "findTag", "findTags",
		"markerWall", "sceneWall", "markerStrings", "stats", "sceneMarkerTags",
		"allScenes", "allSceneMarkers", "allImages", "allGalleries",
		"allPerformers", "allTags", "allStudios", "allMovies",
		// needed by the UI for every user
		"configuration", "systemStatus", "me", "version", "latestversion",
		"jobQueue", "findJob", "plugins", "listScrapers",
	),
	rules(user.RoleEditor,
		"findDuplicateScenes", "findScenesByPathRegex", "parseSceneFilenames",
		"scrapeSingleScene", "scrapeMultiScenes", "scrapeSingleStudio",
		"scrapeSingleTag", "scrapeSinglePerformer", "scrapeMultiPerformers",
		"scrapeSingleGallery", "scrapeSingleMovie", "scrapeSingleGroup",
		"scrapeSingleImage", "scrapeURL", "scrapePerformerURL", "scrapeSceneURL",
		"scrapeGalleryURL", "scrapeImageURL", "scrapeMovieURL", "scrapeGroupURL",
	),
	rules(user.RoleAdmin,
		"logs", "pluginTasks", "installedPackages", "availablePackages",
		"directory", "validateStashBoxCredentials", "findUsers", "dlnaStatus",
	),
)

// ratingOnly allows viewers to use an update mutation if the input only
// changes the rating.
var ratingOnly = fieldRule{Role: user.RoleViewer, Check: checkRatingOnly}

var mutationRules = merge(
	rules(user.RoleViewer,
		// per-user activity
		"sceneIncrementO", "sceneDecrementO", "sceneAddO", "sceneDeleteO", "sceneResetO",
		"sceneSaveActivity", "sceneResetActivity", "sceneIncrementPlayCount",
		"sceneAddPlay", "sceneDeletePlay", "sceneResetPlayCount",
		"imageIncrementO", "imageDecrementO", "imageResetO",
		// per-user settings
		"saveFilter", "destroySavedFilter", "setDefaultFilter",
		"configureUI", "configureUISetting", "generateAPIKey", "changePassword",
	),
	map[string]fieldRule{
		// viewers may only change ratings
		"sceneUpdate": ratingOnly, "bulkSceneUpdate": ratingOnly, "scenesUpdate": ratingOnly,
		"imageUpdate": ratingOnly, "bulkImageUpdate": ratingOnly, "imagesUpdate": ratingOnly,
		"galleryUpdate": ratingOnly, "bulkGalleryUpdate": ratingOnly, "galleriesUpdate": ratingOnly,
		"performerUpdate": ratingOnly, "bulkPerformerUpdate": ratingOnly,
		"studioUpdate": ratingOnly, "bulkStudioUpdate": ratingOnly,
		"movieUpdate": ratingOnly, "bulkMovieUpdate": ratingOnly,
		"groupUpdate": ratingOnly, "bulkGroupUpdate": ratingOnly,
		// deleting files from disk requires the admin role
		"sceneDestroy":   {Role: user.RoleEditor, Check: checkNoFileDeletion},
		"scenesDestroy":  {Role: user.RoleEditor, Check: checkNoFileDeletion},
		"imageDestroy":   {Role: user.RoleEditor, Check: checkNoFileDeletion},
		"imagesDestroy":  {Role: user.RoleEditor, Check: checkNoFileDeletion},
		"galleryDestroy": {Role: user.RoleEditor, Check: checkNoFileDeletion},
	},
	rules(user.RoleEditor,
		"sceneCreate", "sceneMerge", "sceneGenerateScreenshot",
		"sceneMarkerCreate", "sceneMarkerUpdate", "bulkSceneMarkerUpdate",
		"sceneMarkerDestroy", "sceneMarkersDestroy",
		"galleryCreate", "addGalleryImages", "removeGalleryImages",
		"setGalleryCover", "resetGalleryCover",
		"galleryChapterCreate", "galleryChapterUpdate", "galleryChapterDestroy",
		"performerCreate", "performerDestroy", "performersDestroy", "performerMerge",
		"studioCreate", "studioDestroy", "studiosDestroy",
		"movieCreate", "movieDestroy", "moviesDestroy",
		"groupCreate", "groupDestroy", "groupsDestroy",
		"addGroupSubGroups", "removeGroupSubGroups", "reorderSubGroups",
		"tagCreate", "tagUpdate", "tagDestroy", "tagsDestroy", "tagsMerge", "bulkTagUpdate",
		"submitStashBoxFingerprints", "submitStashBoxSceneDraft", "submitStashBoxPerformerDraft",
		"stashBoxBatchPerformerTag", "stashBoxBatchStudioTag", "stashBoxBatchTagTag",
	),
	rules(user.RoleAdmin,
		"setup", "migrate", "downloadFFMpeg", "sceneAssignFile",
		"moveFiles", "deleteFiles", "destroyFiles", "fileSetFingerprints",
		"revealFileInFileManager", "revealFolderInFileManager",
		"configureGeneral", "configureInterface", "configureDLNA", "configureScraping",
		"configureDefaults", "configurePlugin",
		"userCreate", "userUpdate", "userDestroy", "userGenerateAPIKey",
		"exportObjects", "importObjects", "metadataImport", "metadataExport",
		"metadataScan", "scanFile", "metadataGenerate", "metadataAutoTag", "metadataClean",
		"metadataCleanGenerated", "metadataIdentify",
		"migrateHashNaming", "migrateSceneScreenshots", "migrateBlobs",
		"anonymiseDatabase", "optimiseDatabase", "backupDatabase", "querySQL", "execSQL",
		"reloadScrapers", "setPluginsEnabled", "runPluginTask", "runPluginOperation", "reloadPlugins",
		"installPackages", "updatePackages", "uninstallPackages",
		"stopJob", "stopAllJobs",
		"enableDLNA", "disableDLNA", "addTempDLNAIP", "removeTempDLNAIP",
	),
)

var subscriptionRules = merge(
	rules(user.RoleViewer, "jobsSubscribe", "scanCompleteSubscribe"),
	rules(user.RoleAdmin, "loggingSubscribe"),
)

// ratingFields are the input fields a viewer may set in update mutations.
var ratingFields = map[string]bool{
	"id":        true,
	"ids":       true,
	"rating":    true,
	"rating100": true,
	// client mutation id, ignored
	"clientMutationId": true,
}

func checkRatingOnly(u user.User, args map[string]interface{}) error {
	if u.Role.AtLeast(user.RoleEditor) {
		return nil
	}

	inputs := []interface{}{args["input"]}
	if list, ok := args["input"].([]interface{}); ok {
		inputs = list
	}

	for _, in := range inputs {
		m, ok := in.(map[string]interface{})
		if !ok {
			return fmt.Errorf("%w: invalid input", ErrForbidden)
		}
		for k := range m {
			if !ratingFields[k] {
				return fmt.Errorf("%w: role %s may only change the rating (field %q not allowed)", ErrForbidden, u.Role, k)
			}
		}
	}
	return nil
}

// fileDeletionFields are destroy input fields that delete data from disk.
var fileDeletionFields = []string{"delete_file", "delete_generated", "destroy_file_entry"}

func checkNoFileDeletion(u user.User, args map[string]interface{}) error {
	if u.IsAdmin() {
		return nil
	}

	m, _ := args["input"].(map[string]interface{})
	for _, f := range fileDeletionFields {
		if v, ok := m[f].(bool); ok && v {
			return fmt.Errorf("%w: deleting files requires role %s", ErrForbidden, user.RoleAdmin)
		}
	}
	return nil
}

// authorizeRootField checks whether the user may use the given root field.
func authorizeRootField(u user.User, object string, field string, args map[string]interface{}) error {
	objRules, ok := rootFieldRules[object]
	if !ok {
		return nil
	}

	rule, ok := objRules[field]
	if !ok {
		// deny by default
		rule = fieldRule{Role: user.RoleAdmin}
	}

	if !u.Role.AtLeast(rule.Role) {
		return fmt.Errorf("%w: %s requires role %s", ErrForbidden, field, rule.Role)
	}

	if rule.Check != nil {
		return rule.Check(u, args)
	}
	return nil
}

// authorizationMiddleware is a gqlgen field middleware enforcing the
// permissions of the current user on all root fields.
func authorizationMiddleware(ctx context.Context, next graphql.Resolver) (interface{}, error) {
	fc := graphql.GetFieldContext(ctx)
	if fc == nil || !isRootObject(fc.Object) {
		return next(ctx)
	}

	u := user.Current(ctx)

	var args map[string]interface{}
	if oc := graphql.GetOperationContext(ctx); oc != nil && fc.Field.Field != nil {
		args = fc.Field.ArgumentMap(oc.Variables)
	}

	if err := authorizeRootField(u, fc.Object, fc.Field.Name, args); err != nil {
		logger.Warnf("user %q (%s) denied %s.%s", u.Username, u.Role, fc.Object, fc.Field.Name)
		return nil, err
	}

	return next(ctx)
}

func isRootObject(o string) bool {
	return o == rootQuery || o == rootMutation || o == rootSubscription
}

// requireRoleHandler is HTTP middleware rejecting users without the role.
func requireRoleHandler(role user.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := requireRole(r.Context(), role); err != nil {
				http.Error(w, err.Error(), http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
