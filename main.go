package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
	"sigs.k8s.io/structured-merge-diff/v4/typed"
)

func main() {
	kubeconfig := "$HOME/.kube/config"
	config, err := clientcmd.BuildConfigFromFlags("", os.ExpandEnv(kubeconfig))
	if err != nil {
		fmt.Printf("Error building kubeconfig: %v\n", err)
		log.Fatal(err)
	}
	client := dynamic.NewForConfigOrDie(config)

	resource, err := client.Resource(
		schema.GroupVersionResource{
			Group:    "apps",
			Version:  "v1",
			Resource: "deployments",
		},
	).
		Namespace("default").
		Get(context.Background(), "nginx-deployment", metav1.GetOptions{})

	if err != nil {
		log.Fatal(err)
	}
	mfs := resource.GetManagedFields()
	for _, mf := range mfs {
		fieldset := &fieldpath.Set{}
		if mf.Manager != "kubectl-client-side-apply" {
			continue
		}
		fs := &fieldpath.Set{}
		err = fs.FromJSON(bytes.NewReader(mf.FieldsV1.Raw))
		if err != nil {
			log.Fatal(err)
		}

		fieldset = fieldset.Union(fs)

		d, err := typed.DeducedParseableType.FromUnstructured(resource.Object)
		if err != nil {
			log.Fatal(err)
		}

		x := d.ExtractItems(fieldset.Leaves()).AsValue().Unstructured()
		m, ok := x.(map[string]any)
		if !ok {
			log.Fatal("cannot cast")
		}

		m2 := colorizeFields(m)

		j, err := json.MarshalIndent(m2, "", "  ")
		if err != nil {
			log.Fatal(err)
		}

		c := strings.ReplaceAll(string(j), "\"$(RED)", "\033[31m\"")
		c = strings.ReplaceAll(c, "\"$(RESET)", "\033[00m\"")
		c = strings.ReplaceAll(c, " }", "\033[00m }")
		fmt.Println(c)
	}
}

func colorizeFields(fields map[string]any) map[string]any {
	colorized := map[string]any{}
	recursiveColorize(fields, colorized)

	return colorized
}

func recursiveColorize(fields, colorized map[string]any) {
	for key, value := range fields {
		switch typedValue := value.(type) {
		case map[string]any:
			keyWithColor := fmt.Sprintf("$(RESET)%s", key)
			child := map[string]any{}
			colorized[keyWithColor] = child
			recursiveColorize(typedValue, child)
		case []any:
			_, ok := typedValue[0].(map[string]any)
			if !ok {
				keyWithColor := fmt.Sprintf("$(RED)%s", key)
				colorized[keyWithColor] = typedValue
				break
			}

			keyWithColor := fmt.Sprintf("$(RESET)%s", key)
			var children []map[string]any
			colorized[keyWithColor] = children
			for _, tv := range typedValue {
				child := map[string]any{}
				colorized[keyWithColor] = append(colorized[keyWithColor].([]map[string]any), child)
				recursiveColorize(tv.(map[string]any), child)
			}
		default:
			keyWithColor := fmt.Sprintf("$(RED)%s", key)
			colorized[keyWithColor] = typedValue
		}
	}
}
