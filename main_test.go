package main

import (
	"testing"

	v1 "k8s.io/api/core/v1"
)

func TestPatchPodSpec(t *testing.T) {
	// Define a sample PodSpec
	podSpec := &v1.PodSpec{
		Containers: []v1.Container{
			{
				Name:  "container-1",
				Image: "nginx:1.14",
			},
		},
		InitContainers: []v1.Container{
			{
				Name:  "init-container-1",
				Image: "busybox:1.28",
			},
		},
	}

	expectedPatch := `[{"op":"add","path":"/metadata/annotations/image-container_1","value":"nginx:1.14"},{"op":"add","path":"/metadata/annotations/image-init_container_1","value":"busybox:1.28"}]`

	patch, err := patchPodSpec(podSpec, false)
	if err != nil {
		t.Errorf("patchPodSpec returned an error: %v", err)
	}

	// Convert the patch to a string
	patchStr := string(patch)

	// Compare the generated patch with the expected patch
	if patchStr != expectedPatch {
		t.Errorf("Generated patch does not match the expected patch.\nExpected: %s\nGot: %s", expectedPatch, patchStr)
	}
}
