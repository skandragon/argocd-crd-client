package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	argoclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

const (
	namespace = "argocd"
	cmName    = "argocd-rbac-cm"
)

func main() {
	policies := []string{}

	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// create the kubeClientset
	kubeClientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// create the Argo clientset
	argoClientset, err := argoclientset.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	cmPolicies, err := fetchConfigmap(context.TODO(), kubeClientset)
	if err != nil {
		panic(err.Error())
	}
	policies = append(policies, cmPolicies...)

	cms, err := argoClientset.ArgoprojV1alpha1().AppProjects(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	for _, p := range cms.Items {
		for _, role := range p.Spec.Roles {
			roleName := fmt.Sprintf("proj:%s:%s", p.Name, role.Name)
			for _, policy := range role.Policies {
				fields := strings.Split(policy, ",")
				if len(fields) != 6 {
					continue
				}
				foundSubject := strings.TrimSpace(fields[1])
				if foundSubject != roleName {
					continue
				}
				foundObject := strings.TrimSpace(fields[4][:len(p.Name)+2])
				if foundObject != p.Name+"/" {
					continue
				}
				policies = append(policies, policy)
			}
			for _, group := range role.Groups {
				policies = append(policies, fmt.Sprintf("g, %s, %s", group, roleName))
			}
		}
	}

	for _, policy := range policies {
		fmt.Printf("%s\n", policy)
	}
}

func fetchConfigmap(ctx context.Context, clientset *kubernetes.Clientset) ([]string, error) {
	// Examples for error handling:
	// - Use helper functions like e.g. errors.IsNotFound()
	// - And/or cast to StatusError and use its properties like e.g. ErrStatus.Message
	cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(ctx, cmName, metav1.GetOptions{})
	if err != nil {
		return []string{}, err
	}

	policyData, found := cm.Data["policy.csv"]
	if !found {
		return []string{}, fmt.Errorf("unable to find policy.csv in configmap %s", cmName)
	}
	policies := strings.Split(policyData, "\n")
	filtered := []string{}
	for _, policy := range policies {
		p := strings.TrimSpace(policy)
		if p != "" {
			filtered = append(filtered, p)
		}
	}
	return filtered, nil
}
