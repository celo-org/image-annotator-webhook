package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var deploymentTemplate = &appsv1.Deployment{
	ObjectMeta: metav1.ObjectMeta{
		Name: "test-deployment",
		Annotations: map[string]string{
			"image.clabs.co/container-1":      "nginx:1.14",
			"image.clabs.co/init-container-1": "busybox:1.28",
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

	expectedPatch := `[{"op":"add","path":"/metadata/annotations","value":{}},{"op":"add","path":"/metadata/annotations/image.clabs.co~1container-1","value":"nginx:1.14"},{"op":"add","path":"/metadata/annotations/image.clabs.co~1init-container-1","value":"busybox:1.28"}]`

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

	expectedPatch := `[{"op":"add","path":"/metadata/annotations/image.clabs.co~1init-container-1","value":"busybox:1.28"}]`

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
		"image.clabs.co/container-1": "nginx:1.14",
	}
	expectedPatch := `[{"op":"add","path":"/metadata/annotations/image.clabs.co~1init-container-1","value":"busybox:1.28"}]`

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
func TestMutateHandler(t *testing.T) {
	// Define a sample AdmissionReview object
	admissionReview := admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			Object: runtime.RawExtension{
				Raw: []byte(`{"kind":"Pod","apiVersion":"v1","metadata":{"name":"test-pod","namespace":"test-namespace","annotations":{"image.clabs.co/test-container":"nginx:1.13"}},"spec":{"containers":[{"name":"test-container","image":"nginx:1.14"}]}}`),
			},
		},
	}

	admissionReviewBytes, err := json.Marshal(admissionReview)
	if err != nil {
		t.Fatalf("Failed to marshal AdmissionReview object: %v", err)
	}

	req, err := http.NewRequest("POST", "/mutate", bytes.NewBuffer(admissionReviewBytes))
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Create a new HTTP response recorder
	rr := httptest.NewRecorder()

	// Call the mutateHandler function
	handler := http.HandlerFunc(mutateHandler)
	handler.ServeHTTP(rr, req)

	// Check the response status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v, want %v", status, http.StatusOK)
	}

	// Check the response body
	var admissionReviewResponse admissionv1.AdmissionReview
	if err := json.Unmarshal(rr.Body.Bytes(), &admissionReviewResponse); err != nil {
		t.Errorf("Failed to unmarshal response body: %v", err)
	}
	if admissionReviewResponse.Response.Allowed != true {
		t.Errorf("Handler returned wrong response: got %v, want %v", admissionReviewResponse.Response.Allowed, true)
	}
}

func TestMutateHandlerDeployment(t *testing.T) {
	// Define a sample AdmissionReview object
	admissionReview := admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			Object: runtime.RawExtension{
				Raw: []byte(`{"kind":"Deployment","apiVersion":"apps/v1","metadata":{"name":"test-deployment","namespace":"test-namespace","annotations":{"image.clabs.co/container-1":"nginx:1.13","image.clabs.co/init-container-1":"busybox:1.28"}},"spec":{"template":{"spec":{"containers":[{"name":"container-1","image":"nginx:1.14"}],"initContainers":[{"name":"init-container-1","image":"busybox@sha256:01d754b407dd0ec69e52a224cbf35272c8ba6a3fdd9ef512560fd65111995305"}]}}}}`),
			},
		},
	}

	admissionReviewBytes, err := json.Marshal(admissionReview)
	if err != nil {
		t.Fatalf("Failed to marshal AdmissionReview object: %v", err)
	}

	req, err := http.NewRequest("POST", "/mutate", bytes.NewBuffer(admissionReviewBytes))
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Create a new HTTP response recorder
	rr := httptest.NewRecorder()

	// Call the mutateHandler function
	handler := http.HandlerFunc(mutateHandler)
	handler.ServeHTTP(rr, req)

	// Check the response status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v, want %v", status, http.StatusOK)
	}

	// Check the response body
	var admissionReviewResponse admissionv1.AdmissionReview
	if err := json.Unmarshal(rr.Body.Bytes(), &admissionReviewResponse); err != nil {
		t.Errorf("Failed to unmarshal response body: %v", err)
	}
	if admissionReviewResponse.Response.Allowed != true {
		t.Errorf("Handler returned wrong response: got %v, want %v", admissionReviewResponse.Response.Allowed, true)
	}
}

func TestMutateHandlerInvalidRequestBody(t *testing.T) {
	// Create a new HTTP request with an invalid request body
	req, err := http.NewRequest("POST", "/mutate", bytes.NewBuffer([]byte(`invalid-json`)))
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Create a new HTTP response recorder
	rr := httptest.NewRecorder()

	// Call the mutateHandler function
	handler := http.HandlerFunc(mutateHandler)
	handler.ServeHTTP(rr, req)

	// Check the response status code
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("Handler returned wrong status code: got %v, want %v", status, http.StatusBadRequest)
	}
}

func TestMutateHandlerInvalidObject(t *testing.T) {
	// Define a sample AdmissionReview object with an invalid object
	admissionReview := admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			Object: runtime.RawExtension{
				Raw: []byte(`{"kind":"InvalidKind","apiVersion":"v1","metadata":{"name":"test-pod","namespace":"test-namespace"},"spec":{"containers":[{"name":"test-container","image":"nginx:1.14"}]}}`),
			},
		},
	}
	admissionReviewBytes, err := json.Marshal(admissionReview)
	if err != nil {
		t.Fatalf("Failed to marshal AdmissionReview object: %v", err)
	}

	// Create a new HTTP request with the AdmissionReview object as the request body
	req, err := http.NewRequest("POST", "/mutate", bytes.NewBuffer([]byte{}))
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Body = ioutil.NopCloser(bytes.NewBuffer(admissionReviewBytes))

	// Create a new HTTP response recorder
	rr := httptest.NewRecorder()

	// Call the mutateHandler function
	handler := http.HandlerFunc(mutateHandler)
	handler.ServeHTTP(rr, req)

	// Check the response status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v, want %v", status, http.StatusOK)
	}

	// Check the response body
	var admissionReviewResponse admissionv1.AdmissionReview
	if err := json.Unmarshal(rr.Body.Bytes(), &admissionReviewResponse); err != nil {
		t.Errorf("Failed to unmarshal response body: %v", err)
	}
	if admissionReviewResponse.Response.Allowed != false {
		t.Errorf("Handler returned wrong response: got %v, want %v", admissionReviewResponse.Response.Allowed, false)
	}
}

func TestMutateHandlerObjectWithoutPodSpec(t *testing.T) {
	// Define a sample AdmissionReview object with an object that does not have a PodSpec
	admissionReview := admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			Object: runtime.RawExtension{
				Raw: []byte(`{"kind":"Service","apiVersion":"v1","metadata":{"name":"test-service","namespace":"test-namespace"}}`),
			},
		},
	}

	admissionReviewBytes, err := json.Marshal(admissionReview)
	if err != nil {
		t.Fatalf("Failed to marshal AdmissionReview object: %v", err)
	}

	// Create a new HTTP request with the AdmissionReview object as the request body
	req, err := http.NewRequest("POST", "/mutate", bytes.NewBuffer([]byte{}))
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Body = ioutil.NopCloser(bytes.NewBuffer(admissionReviewBytes))

	// Create a new HTTP response recorder
	rr := httptest.NewRecorder()

	// Call the mutateHandler function
	handler := http.HandlerFunc(mutateHandler)
	handler.ServeHTTP(rr, req)

	// Check the response status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v, want %v", status, http.StatusOK)
	}

	// Check the response body
	var admissionReviewResponse admissionv1.AdmissionReview
	if err := json.Unmarshal(rr.Body.Bytes(), &admissionReviewResponse); err != nil {
		t.Errorf("Failed to unmarshal response body: %v", err)
	}
	if admissionReviewResponse.Response.Allowed != true {
		t.Errorf("Handler returned wrong response: got %v, want %v", admissionReviewResponse.Response.Allowed, true)
	}
}
