package kind

import (
	"encoding/json"
	"fmt"
	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"strings"
)

const (
	admissionWebhookAnnotationValidateKey = "webhook-demo.gox.com/validate"
	admissionWebhookAnnotationMutateKey   = "webhook-demo.gox.com/mutate"
	admissionWebhookAnnotationStatusKey   = "webhook-demo.gox.com/status"

	nameLabel      = "app.kubernetes.io/name"
	instanceLabel  = "app.kubernetes.io/instance"
	versionLabel   = "app.kubernetes.io/version"
	componentLabel = "app.kubernetes.io/component"
	partOfLabel    = "app.kubernetes.io/part-of"
	managedByLabel = "app.kubernetes.io/managed-by"

	NA = "not_available"

	webhookNamespace = "webhook-demo"
)

var (
	checkedNamespaces = []string{
		webhookNamespace,
	}

	// service & deployment
	requiredLabels = []string{
		nameLabel,
		instanceLabel,
		versionLabel,
		componentLabel,
		partOfLabel,
		managedByLabel,
	}
	addLabels = map[string]string{
		nameLabel:      NA,
		instanceLabel:  NA,
		versionLabel:   NA,
		componentLabel: NA,
		partOfLabel:    NA,
		managedByLabel: NA,
	}

	// pod
	podRequiredLabels = []string{
		nameLabel,
	}
	addPodLabels = map[string]string{
		nameLabel: NA,
	}
)

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func updateAnnotation(target map[string]string, added map[string]string) (patch []patchOperation) {
	for key, value := range added {
		if target == nil || target[key] == "" {
			target = map[string]string{}
			patch = append(patch, patchOperation{
				Op:   "add",
				Path: "/metadata/annotations",
				Value: map[string]string{
					key: value,
				},
			})
		} else {
			patch = append(patch, patchOperation{
				Op:    "replace",
				Path:  "/metadata/annotations/" + strings.ReplaceAll(key, "/", "~1"),
				Value: value,
			})
		}
	}
	return patch
}

func updateLabels(target map[string]string, added map[string]string) (patch []patchOperation) {
	newValues := make(map[string]string)
	// only update label version
	updateValues := make(map[string]string)
	for key, value := range added {
		if target == nil || target[key] == "" {
			newValues[key] = value
		} else if key == versionLabel {
			updateValues[key] = "v1.0"
		}
	}
	klog.Infof("update label:%+v", updateValues)
	if len(newValues) != 0 {
		patch = append(patch, patchOperation{
			Op:    "add",
			Path:  "/metadata/labels",
			Value: newValues,
		})
	}
	// fix patch key with /: use ~1 to encode /
	for k, v := range updateValues {
		patch = append(patch, patchOperation{
			Op:    "replace",
			Path:  "/metadata/labels/" + strings.ReplaceAll(k, "/", "~1"),
			Value: v,
		})
	}

	return patch
}

func createPatch(availableAnnotations map[string]string, annotations map[string]string, availableLabels map[string]string, labels map[string]string) ([]byte, error) {
	var patch []patchOperation
	klog.Infof("availableAnnotations: %+v, annotations:%+v", availableAnnotations, annotations)
	klog.Infof("availableLabels: %+v, labels:%+v", availableLabels, labels)
	patch = append(patch, updateAnnotation(availableAnnotations, annotations)...)
	patch = append(patch, updateLabels(availableLabels, labels)...)

	return json.Marshal(patch)
}

func admissionRequired(checkkedList []string, admissionAnnotationKey string, metadata *metav1.ObjectMeta) bool {
	required := false
	// skip special kubernetes system namespaces
	for _, namespace := range checkkedList {
		if metadata.Namespace == namespace {
			required = true
		}
	}

	annotations := metadata.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	switch strings.ToLower(annotations[admissionAnnotationKey]) {
	case "n", "no", "false", "off":
		required = false
	}
	return required
}

func mutationRequired(checkedList []string, metadata *metav1.ObjectMeta) bool {
	required := admissionRequired(checkedList, admissionWebhookAnnotationMutateKey, metadata)
	annotations := metadata.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	status := annotations[admissionWebhookAnnotationStatusKey]

	if strings.ToLower(status) == "mutated" {
		required = false
	}

	klog.Infof("Mutation policy for %v/%v: required:%v", metadata.Namespace, metadata.Name, required)
	return required
}

func Addlabel(ar admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	req := ar.Request
	var (
		availableLabels, availableAnnotations map[string]string
		objectMeta                            *metav1.ObjectMeta
		resourceNamespace, resourceName       string
	)

	klog.Infof("======begin Mutating Admission for Namespace=[%v], Kind=[%v], Name=[%v]======", req.Namespace, req.Kind.Kind, req.Name)

	switch req.Kind.Kind {
	case "Deployment":
		var deployment appsv1.Deployment
		if err := json.Unmarshal(req.Object.Raw, &deployment); err != nil {
			klog.Infof("Could not unmarshal raw object: %v", err)
			return &admissionv1.AdmissionResponse{
				Result: &metav1.Status{
					Message: err.Error(),
				},
			}
		}
		resourceName, resourceNamespace, objectMeta = deployment.Name, deployment.Namespace, &deployment.ObjectMeta
		availableLabels = deployment.Labels
		availableAnnotations = deployment.Annotations
	case "Service":
		var service corev1.Service
		if err := json.Unmarshal(req.Object.Raw, &service); err != nil {
			klog.Infof("Could not unmarshal raw object: %v", err)
			return &admissionv1.AdmissionResponse{
				Result: &metav1.Status{
					Message: err.Error(),
				},
			}
		}
		resourceName, resourceNamespace, objectMeta = service.Name, service.Namespace, &service.ObjectMeta
		availableLabels = service.Labels
		availableAnnotations = service.Annotations
	case "Pod":
		var pod corev1.Pod
		if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
			klog.Infof("Could not unmarshal raw object: %v", err)
			return &admissionv1.AdmissionResponse{
				Result: &metav1.Status{
					Message: err.Error(),
				},
			}
		}
		resourceName, resourceNamespace, objectMeta = pod.Name, pod.Namespace, &pod.ObjectMeta
		availableLabels = pod.Labels
		availableAnnotations = pod.Annotations
	//其他不支持的类型
	default:
		msg := fmt.Sprintf("Not support for this Kind of resource  %v", req.Kind.Kind)
		klog.Info(msg)
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: msg,
			},
		}
	}

	if !mutationRequired(checkedNamespaces, objectMeta) {
		klog.Infof("Skipping validation for %s/%s due to policy check", resourceNamespace, resourceName)
		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}

	// add mutate annotation
	annotations := map[string]string{admissionWebhookAnnotationStatusKey: "mutated"}

	var patchBytes []byte
	var err error
	// add labels and annotation
	if req.Kind.Kind == "Pod" {
		patchBytes, err = createPatch(availableAnnotations, annotations, availableLabels, addPodLabels)
		if err != nil {
			return &admissionv1.AdmissionResponse{
				Result: &metav1.Status{
					Message: err.Error(),
				},
			}
		}
	} else {
		patchBytes, err = createPatch(availableAnnotations, annotations, availableLabels, addLabels)
		if err != nil {
			return &admissionv1.AdmissionResponse{
				Result: &metav1.Status{
					Message: err.Error(),
				},
			}
		}
	}

	klog.Infof("AdmissionResponse: patch=%v\n", string(patchBytes))
	return &admissionv1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *admissionv1.PatchType {
			pt := admissionv1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

func validationRequired(checkkedList []string, metadata *metav1.ObjectMeta) bool {
	required := admissionRequired(checkkedList, admissionWebhookAnnotationValidateKey, metadata)
	klog.Infof("Validation policy for %v/%v: required:%v", metadata.Namespace, metadata.Name, required)
	return required
}

func CheckLabel(ar admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	req := ar.Request
	var (
		availableLabels                 map[string]string
		objectMeta                      *metav1.ObjectMeta
		resourceNamespace, resourceName string
	)

	klog.Infof("======begin Validating Admission for Namespace=[%v], Kind=[%v], Name=[%v]======", req.Namespace, req.Kind.Kind, req.Name)

	switch req.Kind.Kind {
	case "Deployment":
		var deployment appsv1.Deployment
		if err := json.Unmarshal(req.Object.Raw, &deployment); err != nil {
			klog.Infof("Could not unmarshal raw object: %v", err)
			return &admissionv1.AdmissionResponse{
				Result: &metav1.Status{
					Message: err.Error(),
				},
			}
		}
		resourceName, resourceNamespace, objectMeta = deployment.Name, deployment.Namespace, &deployment.ObjectMeta
		availableLabels = deployment.Labels
	case "Service":
		var service corev1.Service
		if err := json.Unmarshal(req.Object.Raw, &service); err != nil {
			klog.Infof("Could not unmarshal raw object: %v", err)
			return &admissionv1.AdmissionResponse{
				Result: &metav1.Status{
					Message: err.Error(),
				},
			}
		}
		resourceName, resourceNamespace, objectMeta = service.Name, service.Namespace, &service.ObjectMeta
		availableLabels = service.Labels
	case "Pod":
		var pod corev1.Pod
		if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
			klog.Infof("Could not unmarshal raw object: %v", err)
			return &admissionv1.AdmissionResponse{
				Result: &metav1.Status{
					Message: err.Error(),
				},
			}
		}
		resourceName, resourceNamespace, objectMeta = pod.Name, pod.Namespace, &pod.ObjectMeta
		availableLabels = pod.Labels
	//其他不支持的类型
	default:
		msg := fmt.Sprintf("Not support for this Kind of resource  %v", req.Kind.Kind)
		klog.Info(msg)
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: msg,
			},
		}
	}

	if !validationRequired(checkedNamespaces, objectMeta) {
		klog.Infof("Skipping validation for %s/%s due to policy check", resourceNamespace, resourceName)
		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}

	allowed := true
	var result *metav1.Status
	var requiredLabelsHere []string
	// add labels and annotation
	if req.Kind.Kind == "Pod" {
		requiredLabelsHere = podRequiredLabels
	} else {
		requiredLabelsHere = requiredLabels
	}
	klog.Infof("available labels: %s ", availableLabels)
	klog.Infof("required labels: %s", requiredLabels)

	for _, rl := range requiredLabelsHere {
		if _, ok := availableLabels[rl]; !ok {
			allowed = false
			result = &metav1.Status{
				Reason: "required labels are not set",
			}
			break
		}
	}

	return &admissionv1.AdmissionResponse{
		Allowed: allowed,
		Result:  result,
	}
}
