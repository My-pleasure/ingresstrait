package controllers

import (
	"context"
	"github.com/crossplane/oam-kubernetes-runtime/pkg/oam"
	"github.com/pkg/errors"
	corev1alpha2 "ingresstrait/api/v1alpha2"
	"k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
)

// Reconcile error strings.
const (
	errNotIngressTrait = "object is not a service trait"
)

const LabelKey = "servicetrait.oam.crossplane.io"

var (
	ingressKind       = reflect.TypeOf(v1beta1.Ingress{}).Name()
	ingressAPIVersion = v1beta1.SchemeGroupVersion.String()
)

// IngressInjector adds a Ingress object for the StatefulSet observed in a workload translation.
func IngressInjector(ctx context.Context, trait oam.Trait, objs []oam.Object) ([]oam.Object, error) {
	t, ok := trait.(*corev1alpha2.IngressTrait)
	if !ok {
		return nil, errors.New(errNotIngressTrait)
	}

	if objs == nil {
		return nil, nil
	}

	ingress := &v1beta1.Ingress{
		TypeMeta: metav1.TypeMeta{
			Kind:       ingressKind,
			APIVersion: ingressAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      t.GetName(),
			Namespace: t.GetNamespace(),
			Labels: map[string]string{
				LabelKey: string(t.GetUID()),
			},
		},
		Spec: v1beta1.IngressSpec{
			IngressClassName: t.Spec.Template.IngressClassName,
			Backend:          t.Spec.Template.Backend,
			TLS:              t.Spec.Template.TLS,
			Rules:            t.Spec.Template.Rules,
		},
	}

	objs = append(objs, ingress)

	return objs, nil
}
