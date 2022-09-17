package kind

import (
	"k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// alwaysDeny all requests made to this function.
func AlwaysDeny(ar v1.AdmissionReview) *v1.AdmissionResponse {
	resp := &v1.AdmissionResponse{
		UID:     ar.Request.UID,
		Allowed: false,
		Result: &metav1.Status{
			Message: "always deny for all requests",
		},
	}
	return resp
}
