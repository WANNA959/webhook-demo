package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"

	"github.com/WANNA959/webhook-demo/webhook"
	"github.com/WANNA959/webhook-demo/webhook/kind"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	// TODO: try this library to see if it generates correct json patch
	// https://github.com/mattbaird/jsonpatch
)

// toAdmissionResponse is a helper function to create an AdmissionResponse
// with an embedded error
func toAdmissionResponse(err error) *admissionv1.AdmissionResponse {
	return &admissionv1.AdmissionResponse{
		Result: &metav1.Status{
			Message: err.Error(),
		},
	}
}

// admitFunc is the type we use for all of our validators and mutators
type admitFunc func(admissionv1.AdmissionReview) *admissionv1.AdmissionResponse

// serve handles the http portion of a request prior to handing to an admit
// function
func serve(w http.ResponseWriter, r *http.Request, admit admitFunc) {
	var body []byte
	if r.Body != nil {
		if data, err := io.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		klog.Errorf("contentType=%s, expect application/json", contentType)
		return
	}

	klog.V(2).Info(fmt.Sprintf("handling request: %s", body))

	// The AdmissionReview that was sent to the webhook
	requestedAdmissionReview := admissionv1.AdmissionReview{}

	// The AdmissionReview that will be returned
	responseAdmissionReview := admissionv1.AdmissionReview{}

	deserializer := webhook.Codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(body, nil, &requestedAdmissionReview); err != nil {
		klog.Error(err)
		responseAdmissionReview.Response = toAdmissionResponse(err)
	} else {
		// pass to admitFunc
		responseAdmissionReview.Response = admit(requestedAdmissionReview)
	}

	// Return the same UID
	responseAdmissionReview.Response.UID = requestedAdmissionReview.Request.UID

	// fix: add apiversion+kind
	responseAdmissionReview.APIVersion = "admission.k8s.io/v1"
	responseAdmissionReview.Kind = "AdmissionReview"

	klog.V(2).Info(fmt.Sprintf("sending response: %v", responseAdmissionReview.Response))

	respBytes, err := json.Marshal(responseAdmissionReview)
	if err != nil {
		klog.Error(err)
	}
	if _, err := w.Write(respBytes); err != nil {
		klog.Error(err)
	}
}

func serveAlwaysDeny(w http.ResponseWriter, r *http.Request) {
	serve(w, r, kind.AlwaysDeny)
}

func serveAddLabel(w http.ResponseWriter, r *http.Request) {
	serve(w, r, kind.Addlabel)
}

//
//func servePods(w http.ResponseWriter, r *http.Request) {
//	serve(w, r, admitPods)
//}
//
//func serveAttachingPods(w http.ResponseWriter, r *http.Request) {
//	serve(w, r, denySpecificAttachment)
//}
//
//func serveMutatePods(w http.ResponseWriter, r *http.Request) {
//	serve(w, r, mutatePods)
//}
//
//func serveConfigmaps(w http.ResponseWriter, r *http.Request) {
//	serve(w, r, admitConfigMaps)
//}
//
//func serveMutateConfigmaps(w http.ResponseWriter, r *http.Request) {
//	serve(w, r, mutateConfigmaps)
//}
//
//func serveCustomResource(w http.ResponseWriter, r *http.Request) {
//	serve(w, r, admitCustomResource)
//}
//
//func serveMutateCustomResource(w http.ResponseWriter, r *http.Request) {
//	serve(w, r, mutateCustomResource)
//}
//
//func serveCRD(w http.ResponseWriter, r *http.Request) {
//	serve(w, r, admitCRD)
//}

func main() {
	var config webhook.Config
	config.AddFlags()
	flag.Parse()
	klog.Infof("config:%+v", config)

	http.HandleFunc("/mutate/always-deny", serveAlwaysDeny)
	http.HandleFunc("/mutate/add-label", serveAddLabel)
	//http.HandleFunc("/pods", servePods)
	//http.HandleFunc("/pods/attach", serveAttachingPods)
	//http.HandleFunc("/mutating-pods", serveMutatePods)
	//http.HandleFunc("/configmaps", serveConfigmaps)
	//http.HandleFunc("/mutating-configmaps", serveMutateConfigmaps)
	//http.HandleFunc("/custom-resource", serveCustomResource)
	//http.HandleFunc("/mutating-custom-resource", serveMutateCustomResource)
	//http.HandleFunc("/crd", serveCRD)

	server := &http.Server{
		Addr:      ":443",
		TLSConfig: webhook.ConfigTLS(config),
	}
	server.ListenAndServeTLS("", "")
}
