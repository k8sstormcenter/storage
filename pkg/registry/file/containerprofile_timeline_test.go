package file

import (
	"testing"

	"github.com/kubescape/k8s-interface/instanceidhandler/v1/helpers"
	"github.com/kubescape/storage/pkg/apis/softwarecomposition"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewerTimeline(t *testing.T) {
	tests := []struct {
		name     string
		existing string
		incoming string
		want     string
	}{
		{"both absent", "", "", ""},
		{"first arrival lands", "", "a@2026-01-01T00:00:01Z", "a@2026-01-01T00:00:01Z"},
		{"empty never erases", "a@2026-01-01T00:00:01Z", "", "a@2026-01-01T00:00:01Z"},
		{"later end wins", "a@2026-01-01T00:00:01Z", "a@2026-01-01T00:00:01Z,b@2026-01-01T00:00:02Z", "a@2026-01-01T00:00:01Z,b@2026-01-01T00:00:02Z"},
		{"older incoming loses", "a@2026-01-01T00:00:01Z,b@2026-01-01T00:00:02Z", "a@2026-01-01T00:00:01Z", "a@2026-01-01T00:00:01Z,b@2026-01-01T00:00:02Z"},
		{"equal is idempotent", "a@2026-01-01T00:00:01Z", "a@2026-01-01T00:00:01Z", "a@2026-01-01T00:00:01Z"},
		{"same end, longer wins", "a@2026-01-01T00:00:01Z,b@2026-01-01T00:00:02Z", "a@2026-01-01T00:00:01Z,x,b@2026-01-01T00:00:02Z", "a@2026-01-01T00:00:01Z,x,b@2026-01-01T00:00:02Z"},
		{"unparsable incoming loses", "a@2026-01-01T00:00:01Z", "garbage", "a@2026-01-01T00:00:01Z"},
		{"unparsable existing loses", "garbage", "a@2026-01-01T00:00:01Z", "a@2026-01-01T00:00:01Z"},
		{"both unparsable keeps existing", "garbage", "junk", "garbage"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, newerTimeline(tt.existing, tt.incoming))
		})
	}
}

func tsProfile(annotations map[string]string) *softwarecomposition.ContainerProfile {
	return &softwarecomposition.ContainerProfile{
		ObjectMeta: metav1.ObjectMeta{Annotations: annotations},
	}
}

func TestMergeContainerProfileTS_TimelineIsOrderIndependent(t *testing.T) {
	older := "a@2026-01-01T00:00:01Z"
	newer := "a@2026-01-01T00:00:01Z,b@2026-01-01T00:00:02Z"

	newestFirst := tsProfile(map[string]string{})
	mergeContainerProfileTS(newestFirst, tsProfile(map[string]string{TimelineMetadataKey: newer}))
	mergeContainerProfileTS(newestFirst, tsProfile(map[string]string{TimelineMetadataKey: older}))

	oldestFirst := tsProfile(map[string]string{})
	mergeContainerProfileTS(oldestFirst, tsProfile(map[string]string{TimelineMetadataKey: older}))
	mergeContainerProfileTS(oldestFirst, tsProfile(map[string]string{TimelineMetadataKey: newer}))

	assert.Equal(t, newer, newestFirst.Annotations[TimelineMetadataKey])
	assert.Equal(t, newer, oldestFirst.Annotations[TimelineMetadataKey])
}

func TestMergeContainerProfileTS_TimelineAcrossBatches(t *testing.T) {
	accumulated := tsProfile(map[string]string{TimelineMetadataKey: "a@2026-01-01T00:00:01Z"})

	mergeContainerProfileTS(accumulated, tsProfile(map[string]string{
		TimelineMetadataKey: "a@2026-01-01T00:00:01Z,b@2026-01-01T00:00:02Z",
	}))

	assert.Equal(t, "a@2026-01-01T00:00:01Z,b@2026-01-01T00:00:02Z", accumulated.Annotations[TimelineMetadataKey])
}

func TestMergeContainerProfileTS_TimelineOnNilAnnotations(t *testing.T) {
	accumulated := &softwarecomposition.ContainerProfile{}

	mergeContainerProfileTS(accumulated, tsProfile(map[string]string{TimelineMetadataKey: "a@2026-01-01T00:00:01Z"}))

	assert.Equal(t, "a@2026-01-01T00:00:01Z", accumulated.Annotations[TimelineMetadataKey])
}

func TestMergeContainerProfileTS_OtherAnnotationsStayFirstWriteWins(t *testing.T) {
	accumulated := tsProfile(map[string]string{helpers.WlidMetadataKey: "wlid://first"})

	mergeContainerProfileTS(accumulated, tsProfile(map[string]string{helpers.WlidMetadataKey: "wlid://second"}))

	assert.Equal(t, "wlid://first", accumulated.Annotations[helpers.WlidMetadataKey])
}
