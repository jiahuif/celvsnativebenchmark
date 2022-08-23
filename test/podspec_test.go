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
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kube-openapi/pkg/common"
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

	structural, err := schema.NewStructural(podSpecDef)

	if err != nil {
		b.Fatal(err)
	}

	structural.Extensions.XValidations = v1.ValidationRules{
		v1.ValidationRule{Rule: "has(self.containers)", Message: "containers"},
		v1.ValidationRule{Rule: "has(self.restartPolicy) && (self.restartPolicy == 'Always' || self.restartPolicy == 'OnFailure' || self.restartPolicy == 'Never')", Message: "restartPolicy"},
	}

	v := cel.NewValidator(structural, math.MaxInt64)

	for i := 0; i < b.N; i++ {
		p := toUnstructured(&corev1.PodSpec{HostNetwork: true, RestartPolicy: corev1.RestartPolicyAlways, Containers: []corev1.Container{}})
		errs, _ := v.Validate(context.Background(), field.NewPath("root"), structural, p, p, math.MaxInt64)
		if len(errs) != 0 {
			b.Errorf("unexpected errors: %v", errs)
		}
	}
}

func BenchmarkPodSpecNative(b *testing.B) {
	for i := 0; i < b.N; i++ {
		errList := validation.ValidatePodSpec(&apiscore.PodSpec{RestartPolicy: "+invalid"}, nil, nil, validation.PodValidationOptions{})
		if len(errList) == 0 {
			b.Errorf("empty errList")
		}
	}
}

func toJSONSchemaProps(in any) (*apiextensions.JSONSchemaProps, error) {
	b := new(bytes.Buffer)
	err := json.NewEncoder(b).Encode(in)
	if err != nil {
		return nil, err
	}
	s := new(v1.JSONSchemaProps)
	err = json.NewDecoder(b).Decode(s)
	if err != nil {
		return nil, err
	}
	out := new(apiextensions.JSONSchemaProps)
	err = v1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(s, out, nil)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func loadSchema(path string) (*apiextensions.JSONSchemaProps, error) {
	defs := generatedopenapi.GetOpenAPIDefinitions(func(path string) spec.Ref {
		return spec.MustCreateRef(path)
	})
	s := defs[path].Schema
	err := resolveRefs(defs, &s)
	if err != nil {
		return nil, err
	}
	return toJSONSchemaProps(s)
}

func resolveRefs(defs map[string]common.OpenAPIDefinition, s *spec.Schema) error {
	if s.Ref.GetURL() != nil {
		*s = defs[s.Ref.String()].Schema
	}

	if s.Items != nil {
		if s.Items.Schema != nil {
			err := resolveRefs(defs, s.Items.Schema)
			if err != nil {
				return err
			}
		}
	}

	for n, p := range s.Properties {
		err := resolveRefs(defs, &p)
		if err != nil {
			return err
		}
		s.Properties[n] = p
	}

	if s.AdditionalProperties != nil && s.AdditionalProperties.Schema != nil {
		err := resolveRefs(defs, s.AdditionalProperties.Schema)
		if err != nil {
			return err
		}
	}
	return nil
}

func toUnstructured(whatever any) map[string]interface{} {
	b := new(bytes.Buffer)
	_ = json.NewEncoder(b).Encode(whatever)
	res := make(map[string]interface{})
	_ = json.NewDecoder(b).Decode(&res)
	return res
}
