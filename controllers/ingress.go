package controllers

import (
	"context"
	"github.com/crossplane/oam-kubernetes-runtime/pkg/oam"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"reflect"
)

var (
	ingressKind       = reflect.TypeOf(v1beta1.Ingress{}).Name()
	ingressAPIVersion = v1beta1.SchemeGroupVersion.String()
)

// IngressInjector adds a Ingress object for the StatefulSet observed in a workload translation.
func IngressInjector(ctx context.Context, t oam.Trait, objs []oam.Object) ([]oam.Object, error) {
	if objs == nil {
		return nil, nil
	}

	for _, o := range objs {
		set, ok := o.(*appsv1.StatefulSet)
		if !ok {
			continue
		}

		// Whether we need to determine here that whether statefulset has been created a service.
		ingress := &v1beta1.Ingress{
			TypeMeta: metav1.TypeMeta{
				Kind:       ingressKind,
				APIVersion: ingressAPIVersion,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      set.GetName() + "-ingress",
				Namespace: set.GetNamespace(),
			},
			Spec: v1beta1.IngressSpec{
				Rules: []v1beta1.IngressRule{
					{
						Host: "ingress.test.com",
						IngressRuleValue: v1beta1.IngressRuleValue{
							HTTP: &v1beta1.HTTPIngressRuleValue{
								Paths: []v1beta1.HTTPIngressPath{
									{
										Path: "/",
										Backend: v1beta1.IngressBackend{
											ServiceName: set.GetName(),
											ServicePort: intstr.IntOrString{IntVal: set.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort},
										},
									},
								},
							},
						},
					},
				},
			},
		}
		objs = append(objs, ingress)
		break
	}

	return objs, nil
}
