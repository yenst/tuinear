package cache

import (
	"reflect"
	"testing"
	"time"

	"github.com/yenst/tuinear/internal/issuefilter"
)

func TestIssueFilterPreferencesRoundTripAndIsolateProfiles(t *testing.T) {
	store := openTestStore(t)
	work := issuefilter.State{
		Assignee:         "viewer-work",
		ExcludedStatuses: []string{"completed", "On hold"},
		ExcludedProjects: []string{"No project"},
	}
	personal := issuefilter.State{Priority: "1"}
	if err := store.SaveIssueFilters(t.Context(), "profile:work", work); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveIssueFilters(t.Context(), "profile:personal", personal); err != nil {
		t.Fatal(err)
	}
	gotWork, err := store.LoadIssueFilters(t.Context(), "profile:work")
	if err != nil || !reflect.DeepEqual(gotWork, work) {
		t.Fatalf("work filters = %#v, %v; want %#v", gotWork, err, work)
	}
	gotPersonal, err := store.LoadIssueFilters(t.Context(), "profile:personal")
	if err != nil || !reflect.DeepEqual(gotPersonal, personal) {
		t.Fatalf("personal filters = %#v, %v; want %#v", gotPersonal, err, personal)
	}
	if err := store.SaveIssueFilters(t.Context(), "profile:work", issuefilter.State{}); err != nil {
		t.Fatal(err)
	}
	gotWork, err = store.LoadIssueFilters(t.Context(), "profile:work")
	if err != nil || !gotWork.Empty() {
		t.Fatalf("cleared work filters = %#v, %v", gotWork, err)
	}
	gotPersonal, err = store.LoadIssueFilters(t.Context(), "profile:personal")
	if err != nil || !reflect.DeepEqual(gotPersonal, personal) {
		t.Fatalf("clearing work changed personal filters: %#v, %v", gotPersonal, err)
	}
}

func TestSavedIssueFiltersSurviveSnapshotReplacement(t *testing.T) {
	store := openTestStore(t)
	want := issuefilter.State{Assignee: "viewer-work", ExcludedStatuses: []string{"completed"}}
	if err := store.SaveIssueFilters(t.Context(), "profile:work", want); err != nil {
		t.Fatal(err)
	}
	dashboard := demoDashboard(t)
	if err := store.Save(t.Context(), "work", dashboard, time.Now()); err != nil {
		t.Fatal(err)
	}
	dashboard.Issues = dashboard.Issues[1:]
	if err := store.Save(t.Context(), "work", dashboard, time.Now()); err != nil {
		t.Fatal(err)
	}
	got, err := store.LoadIssueFilters(t.Context(), "profile:work")
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("filters after snapshot replacement = %#v, %v; want %#v", got, err, want)
	}
}
