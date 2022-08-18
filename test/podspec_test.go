package test

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	"k8s.io/kube-openapi/pkg/validation/spec"
	apiscore "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/core/validation"
	generatedopenapi "k8s.io/kubernetes/pkg/generated/openapi"
)

func BenchmarkPodSpecWithCEL(b *testing.B) {
	podSpecDef, err := loadSchema("k8s.io/api/core/v1.PodSpec")
	if err != nil {
		b.Fatal(err)
	}

	stripRefs(podSpecDef)
	structural, err := schema.NewStructural(podSpecDef)

	if err != nil {
		b.Fatal(err)
	}

	structural.Extensions.XValidations = v1.ValidationRules{
		v1.ValidationRule{Rule: "self.hostNetwork == false", Message: "nothing"},
	}

	r, err := cel.Compile(structural, true, math.MaxInt64)

	if err != nil {
		b.Fatal(err)
	}

	for i := 0; i < b.N; i++ {
		v, e, err := r[0].Program.ContextEval(context.Background(), toUnstructured(&corev1.PodSpec{HostNetwork: true}))
		if err != nil {
			b.Errorf("could not eval: %v", err)
		}
		_, _ = v, e
	}
}

func BenchmarkPodSpecNative(b *testing.B) {
	for i := 0; i < b.N; i++ {
		errList := validation.ValidatePodSpec(&apiscore.PodSpec{}, nil, nil, validation.PodValidationOptions{})
		if len(errList) == 0 {
			b.Errorf("empty errList")
		}
	}
}
func loadSchema(path string) (*apiextensions.JSONSchemaProps, error) {
	defs := generatedopenapi.GetOpenAPIDefinitions(func(path string) spec.Ref {
		// does not matter
		return spec.MustCreateRef(path)
	})
	b := new(bytes.Buffer)
	err := json.NewEncoder(b).Encode(defs[path].Schema)
	if err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	schema := new(apiextensions.JSONSchemaProps)
	err = json.NewDecoder(b).Decode(schema)
	if err != nil {
		return nil, err
	}
	return schema, nil
}

func stripRefs(s *apiextensions.JSONSchemaProps) {
	s.Ref = nil
	if s.Items != nil && s.Items.Schema != nil {
		stripRefs(s.Items.Schema)
	}
	if s.AdditionalProperties != nil && s.AdditionalProperties.Schema != nil {
		stripRefs(s.AdditionalProperties.Schema)
	}
	for f, p := range s.Properties {
		stripRefs(&p)
		s.Properties[f] = p
	}
}

func toUnstructured(whatever any) map[string]interface{} {
	b := new(bytes.Buffer)
	_ = json.NewEncoder(b).Encode(whatever)
	res := make(map[string]interface{})
	_ = json.NewDecoder(b).Decode(&res)
	return map[string]any{"self": res}
}
