package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
)

func main() {
	http.HandleFunc("/mutate", mutateHandler)
	cert := "/etc/image-annotator-webhook/tls/tls.crt"
	key := "/etc/image-annotator-webhook/tls/tls.key"
	port := 8443

	lvl, ok := os.LookupEnv("LOG_LEVEL")
	if !ok {
		lvl = "debug"
	}
	level, err := logrus.ParseLevel(lvl)
	if err != nil {
		level = logrus.DebugLevel
	}
	logrus.SetLevel(level)

	logrus.Print("Listening on port 8443...")
	http.ListenAndServeTLS(fmt.Sprintf(":%d", port), cert, key, nil)
}

func mutateHandler(w http.ResponseWriter, r *http.Request) {
	// Resources with podSpec: replicasets, deployments,
	// pods, cronjobs, jobs, statefulsets, daemonsets
	logger := logrus.WithField("uri", r.RequestURI)
	logger.Debug("received mutating request")

	var admissionResponse *admissionv1.AdmissionResponse

	admissionReview := admissionv1.AdmissionReview{}
	if err := json.NewDecoder(r.Body).Decode(&admissionReview); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Initialize deserializer
	deserializer := scheme.Codecs.UniversalDeserializer()
	raw := admissionReview.Request.Object.Raw

	// Deserialize the raw data into the appropriate Kubernetes object
	obj, _, err := deserializer.Decode(raw, nil, nil)
	if err != nil {
		admissionResponse = toAdmissionResponse(err)
	} else {
		// Extract the PodSpec if available
		var podSpec *v1.PodSpec
		switch o := obj.(type) {
		case *v1.Pod:
			podSpec = &o.Spec
		case *appsv1.Deployment:
			podSpec = &o.Spec.Template.Spec
		case *appsv1.StatefulSet:
			podSpec = &o.Spec.Template.Spec
		case *appsv1.ReplicaSet:
			podSpec = &o.Spec.Template.Spec
		case *appsv1.DaemonSet:
			podSpec = &o.Spec.Template.Spec
		case *batchv1.CronJob:
			podSpec = &o.Spec.JobTemplate.Spec.Template.Spec
		case *batchv1.Job:
			podSpec = &o.Spec.Template.Spec
		}

		// If PodSpec is available, patch it
		if podSpec != nil {
			patch, err := patchPodSpec(podSpec)
			logrus.Debug("patch: ", string(patch))
			if err != nil {
				admissionResponse = toAdmissionResponse(err)
			} else {
				admissionResponse = &admissionv1.AdmissionResponse{
					Allowed: true,
					Patch:   patch,
					PatchType: func() *admissionv1.PatchType {
						pt := admissionv1.PatchTypeJSONPatch
						return &pt
					}(),
				}
			}
		} else {
			// Object does not have a PodSpec, so allow it
			admissionResponse = &admissionv1.AdmissionResponse{
				Allowed: true,
			}
		}
	}

	// Return the AdmissionReview to the API server
	// Log the object admissionResponse as JSON
	jsonString, _ := json.Marshal(&admissionResponse)
	logrus.Debug("admissionResponse: ", string(jsonString))
	admissionReview.Response = admissionResponse
	admissionReview.Response.UID = admissionReview.Request.UID
	if err := json.NewEncoder(w).Encode(admissionReview); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func toAdmissionResponse(err error) *admissionv1.AdmissionResponse {
	return &admissionv1.AdmissionResponse{
		Result: &metav1.Status{
			Message: err.Error(),
		},
	}
}

func patchPodSpec(podSpec *v1.PodSpec) ([]byte, error) {
	patch := []map[string]interface{}{}

	// Extract and append container images
	containers := append(podSpec.Containers, podSpec.InitContainers...)
	for _, container := range containers {
		patch = append(patch, map[string]interface{}{
			"op":    "add",
			"path":  "/metadata/annotations/image-" + sanitize(container.Name),
			"value": container.Image,
		})
	}

	return json.Marshal(patch)
}

func sanitize(name string) string {
	return strings.Replace(name, "-", "_", -1)
}
