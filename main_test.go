package main

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var deploymentTemplate = &appsv1.Deployment{
	ObjectMeta: metav1.ObjectMeta{
		Name: "test-deployment",
		Annotations: map[string]string{
			"image-container_1":      "nginx:1.14",
			"image-init_container_1": "busybox:1.28",
		},
	},
	Spec: appsv1.DeploymentSpec{
		Template: v1.PodTemplateSpec{
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:  "container-1",
						Image: "nginx@sha256:f7988fb6c02e0ce69257d9bd9cf37ae20a60f1df7563c3a2a6abe24160306b8d",
					},
				},
				InitContainers: []v1.Container{
					{
						Name:  "init-container-1",
						Image: "busybox:1.28",
					},
				},
			},
		},
	},
}

func TestPatchPodSpec(t *testing.T) {
	// Define a sample PodSpec
	podSpec := deploymentTemplate.DeepCopy().Spec.Template.Spec
	podSpec.Containers[0].Image = "nginx:1.14"

	expectedPatch := `[{"op":"add","path":"/metadata/annotations","value":{}},{"op":"add","path":"/metadata/annotations/image-container_1","value":"nginx:1.14"},{"op":"add","path":"/metadata/annotations/image-init_container_1","value":"busybox:1.28"}]`

	patch, err := patchPodSpec(&podSpec, nil)
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

func TestPatchPodSpecAlreadyPatched(t *testing.T) {
	deployment := deploymentTemplate.DeepCopy()

	expectedPatch := `[{"op":"add","path":"/metadata/annotations/image-init_container_1","value":"busybox:1.28"}]`

	patch, err := patchPodSpec(&deployment.Spec.Template.Spec, deployment.Annotations)
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

func TestPatchPodSpecPartiallyPatched(t *testing.T) {
	deployment := deploymentTemplate.DeepCopy()
	deployment.ObjectMeta.Annotations = map[string]string{
		"image-container_1": "nginx:1.14",
	}
	expectedPatch := `[{"op":"add","path":"/metadata/annotations/image-init_container_1","value":"busybox:1.28"}]`

	patch, err := patchPodSpec(&deployment.Spec.Template.Spec, deployment.Annotations)
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
