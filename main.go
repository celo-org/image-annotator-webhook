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
	lvl, ok := os.LookupEnv("LOG_LEVEL")
	if !ok {
		lvl = "debug"
	}
	level, err := logrus.ParseLevel(lvl)
	if err != nil {
		level = logrus.DebugLevel
	}
	logrus.SetLevel(level)

	http.HandleFunc("/mutate", mutateHandler)
	port := 8443
	cert := "/etc/image-annotator-webhook/tls/tls.crt"
	if _, err := os.Stat(cert); os.IsNotExist(err) {
		logrus.Fatal("TLS certificate not found. Please mount it to /etc/image-annotator-webhook/tls/tls.crt")
	}
	key := "/etc/image-annotator-webhook/tls/tls.key"
	if _, err := os.Stat(key); os.IsNotExist(err) {
		logrus.Fatal("TLS key not found. Please mount it to /etc/image-annotator-webhook/tls/tls.key")
	}

	logrus.Print("Listening on port 8443...")
	http.ListenAndServeTLS(fmt.Sprintf(":%d", port), cert, key, nil)
}

func mutateHandler(w http.ResponseWriter, r *http.Request) {
	// Resources with podSpec: replicasets, deployments,
	// pods, cronjobs, jobs, statefulsets, daemonsets
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
		logger := logrus.WithField("uri", r.RequestURI)
		// Assert the object to metav1.Object to access its metadata
		metaObj, ok := obj.(metav1.Object)
		if !ok {
			// Handle the error
			logger.Error("Object is not a metav1.Object")
			return
		}
		logger.Debug("received mutating request for ", obj.GetObjectKind().GroupVersionKind().Kind, " ", metaObj.GetName(), " in namespace ", metaObj.GetNamespace())

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
			currentAnnotations := obj.(metav1.Object).GetAnnotations()
			patch, err := patchPodSpec(podSpec, currentAnnotations)
			// logrus.Debug("patch: ", string(patch))
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
	logrus.Trace("admissionResponse: ", string(jsonString))
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

func patchPodSpec(podSpec *v1.PodSpec, currentAnnotations map[string]string) ([]byte, error) {
	patch := []map[string]interface{}{}

	if currentAnnotations == nil {
		logrus.Debug("currentAnnotations is nil. Initializing it to an empty map")
		patch = append(patch, map[string]interface{}{
			"op":    "add",
			"path":  "/metadata/annotations",
			"value": map[string]string{},
		})
	}
	// Extract and append container images
	containers := append(podSpec.Containers, podSpec.InitContainers...)
	for _, container := range containers {
		// Skip if the annotation already exists and image has a '@' in it
		// Reason is this is probably a digest from policyController and we don't want to overwrite the annotation
		annotationKey := "image.clabs.co/" + container.Name
		if _, ok := currentAnnotations[annotationKey]; ok && strings.Contains(container.Image, "@") {
			logrus.Debug("Skipping container ", container.Name, " because it already has an annotation and image has a digest")
			continue
		}
		logrus.Debug("Adding annotation for container ", container.Name, " with image ", container.Image)
		patch = append(patch, map[string]interface{}{
			"op": "add",
			// ~1 is a JSON pointer escape for /
			"path":  "/metadata/annotations/image.clabs.co~1" + container.Name,
			"value": container.Image,
		})
	}

	return json.Marshal(patch)
}
