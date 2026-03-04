package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	labelAllowClusterSwitch = "apps.open-cluster-management.io/allow-cluster-switch"
	annotationResult        = "apps.open-cluster-management.io/allow-cluster-switch-result"
	labelPlacement          = "cluster.open-cluster-management.io/placement"
)

var (
	appSetGVK = schema.GroupVersionKind{
		Group:   "argoproj.io",
		Version: "v1alpha1",
		Kind:    "ApplicationSet",
	}
	applicationGVK = schema.GroupVersionKind{
		Group:   "argoproj.io",
		Version: "v1alpha1",
		Kind:    "Application",
	}
	placementGVK = schema.GroupVersionKind{
		Group:   "cluster.open-cluster-management.io",
		Version: "v1beta1",
		Kind:    "Placement",
	}
	placementDecisionListGVK = schema.GroupVersionKind{
		Group:   "cluster.open-cluster-management.io",
		Version: "v1beta1",
		Kind:    "PlacementDecisionList",
	}
)

type ClusterSwitchReconciler struct {
	client.Client
}

func (r *ClusterSwitchReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	appSet := &unstructured.Unstructured{}
	appSet.SetGroupVersionKind(appSetGVK)
	if err := r.Get(ctx, req.NamespacedName, appSet); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// ── Watch criteria ──────────────────────────────────────────────────
	if appSet.GetLabels()[labelAllowClusterSwitch] != "true" {
		return ctrl.Result{}, nil
	}

	statusResources, found, err := unstructured.NestedSlice(appSet.Object, "status", "resources")
	if err != nil || !found || len(statusResources) != 1 {
		log.V(1).Info("skip: ApplicationSet status.resources count != 1")
		return ctrl.Result{}, nil
	}

	// ── Step 1: Extract Placement via clusterDecisionResource generators ─
	placementName, ok := extractPlacementName(appSet)
	if !ok {
		log.V(1).Info("skip: could not resolve exactly one Placement from generators")
		return ctrl.Result{}, nil
	}

	placement := &unstructured.Unstructured{}
	placement.SetGroupVersionKind(placementGVK)
	if err := r.Get(ctx, types.NamespacedName{
		Name:      placementName,
		Namespace: req.Namespace,
	}, placement); err != nil {
		log.V(1).Info("skip: Placement not found", "placement", placementName)
		return ctrl.Result{}, nil
	}

	// ── Step 2: Resolve PlacementDecision ────────────────────────────────
	pdList := &unstructured.UnstructuredList{}
	pdList.SetGroupVersionKind(placementDecisionListGVK)
	if err := r.List(ctx, pdList,
		client.InNamespace(req.Namespace),
		client.MatchingLabels{labelPlacement: placementName},
	); err != nil {
		return ctrl.Result{}, err
	}
	if len(pdList.Items) != 1 {
		log.V(1).Info("skip: PlacementDecision count != 1", "count", len(pdList.Items))
		return ctrl.Result{}, nil
	}

	// ── Step 3: Validate cluster decisions ───────────────────────────────
	decisions, found, err := unstructured.NestedSlice(pdList.Items[0].Object, "status", "decisions")
	if err != nil || !found || len(decisions) != 1 {
		log.V(1).Info("skip: PlacementDecision decisions count != 1")
		return ctrl.Result{}, nil
	}

	decision, ok := decisions[0].(map[string]interface{})
	if !ok {
		log.V(1).Info("skip: invalid decision format")
		return ctrl.Result{}, nil
	}
	clusterName, ok := decision["clusterName"].(string)
	if !ok || clusterName == "" {
		log.V(1).Info("skip: clusterName missing from decision")
		return ctrl.Result{}, nil
	}

	// ── Step 4: Validate Application on resolved cluster ─────────────────
	appRef, ok := statusResources[0].(map[string]interface{})
	if !ok {
		return ctrl.Result{}, nil
	}
	appName, _ := appRef["name"].(string)
	appNamespace, _ := appRef["namespace"].(string)
	if appName == "" {
		log.V(1).Info("skip: Application name missing in ApplicationSet status.resources")
		return ctrl.Result{}, nil
	}
	if appNamespace == "" {
		appNamespace = req.Namespace
	}

	app := &unstructured.Unstructured{}
	app.SetGroupVersionKind(applicationGVK)
	if err := r.Get(ctx, types.NamespacedName{Name: appName, Namespace: appNamespace}, app); err != nil {
		log.V(1).Info("skip: Application not found", "name", appName, "namespace", appNamespace)
		return ctrl.Result{}, nil
	}

	appResources, found, err := unstructured.NestedSlice(app.Object, "status", "resources")
	if err != nil || !found || len(appResources) != 1 {
		log.V(1).Info("skip: Application status.resources count != 1")
		return ctrl.Result{}, nil
	}

	resource, ok := appResources[0].(map[string]interface{})
	if !ok {
		return ctrl.Result{}, nil
	}
	kind, _ := resource["kind"].(string)
	if kind != "VirtualMachine" {
		log.V(1).Info("skip: resource kind is not VirtualMachine", "kind", kind)
		return ctrl.Result{}, nil
	}

	resourceName, _ := resource["name"].(string)
	resourceNamespace, _ := resource["namespace"].(string)

	// ── Step 5: Annotate ApplicationSet ──────────────────────────────────
	value := fmt.Sprintf("cluster=%s,name=%s,namespace=%s", clusterName, resourceName, resourceNamespace)

	annotations := appSet.GetAnnotations()
	if annotations != nil && annotations[annotationResult] == value {
		return ctrl.Result{}, nil
	}

	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[annotationResult] = value
	appSet.SetAnnotations(annotations)

	if err := r.Update(ctx, appSet); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("annotated ApplicationSet with cluster-switch result", "value", value)
	return ctrl.Result{}, nil
}

// extractPlacementName collects Placement names referenced in
// clusterDecisionResource generators via the well-known OCM label.
// Returns ("", false) when 0 or >1 distinct names are found.
func extractPlacementName(appSet *unstructured.Unstructured) (string, bool) {
	generators, found, err := unstructured.NestedSlice(appSet.Object, "spec", "generators")
	if err != nil || !found {
		return "", false
	}

	seen := make(map[string]struct{})
	for _, g := range generators {
		gMap, ok := g.(map[string]interface{})
		if !ok {
			continue
		}
		cdr, ok := gMap["clusterDecisionResource"].(map[string]interface{})
		if !ok {
			continue
		}
		ls, ok := cdr["labelSelector"].(map[string]interface{})
		if !ok {
			continue
		}
		ml, ok := ls["matchLabels"].(map[string]interface{})
		if !ok {
			continue
		}
		name, ok := ml[labelPlacement].(string)
		if !ok || name == "" {
			continue
		}
		seen[name] = struct{}{}
	}

	if len(seen) != 1 {
		return "", false
	}
	for name := range seen {
		return name, true
	}
	return "", false
}

func (r *ClusterSwitchReconciler) SetupWithManager(mgr ctrl.Manager) error {
	appSet := &unstructured.Unstructured{}
	appSet.SetGroupVersionKind(appSetGVK)

	return ctrl.NewControllerManagedBy(mgr).
		For(appSet, builder.WithPredicates(predicate.NewPredicateFuncs(func(obj client.Object) bool {
			return obj.GetLabels()[labelAllowClusterSwitch] == "true"
		}))).
		Complete(r)
}
