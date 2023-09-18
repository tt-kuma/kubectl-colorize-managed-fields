package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
)

var listKeys map[string][]fieldpath.PathElement

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

	managersForFields := map[string][]string{}
	listKeys = map[string][]fieldpath.PathElement{}
	fsAll := &fieldpath.Set{}
	for _, mf := range resource.GetManagedFields() {
		fieldset := &fieldpath.Set{}
		fs := &fieldpath.Set{}
		err = fs.FromJSON(bytes.NewReader(mf.FieldsV1.Raw))
		if err != nil {
			log.Fatal(err)
		}

		fieldset = fieldset.Union(fs)
		fsAll = fsAll.Union(fs)
		fieldset.Leaves().Iterate(func(p fieldpath.Path) {
			path := p.String()
			if managers, ok := managersForFields[path]; ok {
				managersForFields[path] = append(managers, mf.Manager)
			}
			managersForFields[path] = []string{mf.Manager}
		})
	}

	fsAll.Iterate(func(p fieldpath.Path) {
		fmt.Println(p.String())
		lastPathElement := p[len(p)-1]
		if lastPathElement.FieldName != nil || len(p) < 2 {
			return
		}

		pe := p[0 : len(p)-1]

		if _, ok := listKeys[pe.String()]; !ok {
			listKeys[pe.String()] = []fieldpath.PathElement{}
		}
		listKeys[pe.String()] = append(listKeys[pe.String()], lastPathElement)
	})

	resource.SetManagedFields(nil)
	colorized := colorizeFields(resource.Object)

	j, err := json.MarshalIndent(colorized, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	c := strings.ReplaceAll(string(j), "\"$(RED)", "\033[31m\"")
	c = strings.ReplaceAll(c, "\"$(RESET)", "\033[00m\"")
	c = strings.ReplaceAll(c, " }", "\033[00m }")
	fmt.Println(c)

}

func colorizeFields(fields map[string]any) map[string]any {
	colorized := map[string]any{}
	recursiveColorize(fields, colorized, "")

	return colorized
}

func recursiveColorize(fields, colorized map[string]any, prefix string) error {
	for key, value := range fields {
		switch typedValue := value.(type) {
		case map[string]any:
			keyWithColor := fmt.Sprintf("$(RESET)%s", key)
			child := map[string]any{}
			colorized[keyWithColor] = child
			recursiveColorize(typedValue, child, fmt.Sprintf("%s.%s", prefix, key))
		case []any:
			if len(typedValue) == 0 {
				keyWithColor := fmt.Sprintf("$(RED)%s", key)
				colorized[keyWithColor] = typedValue
				break
			}

			if _, ok := typedValue[0].(map[string]any); !ok {
				keyWithColor := fmt.Sprintf("$(RED)%s", key)
				colorized[keyWithColor] = typedValue
				break
			}

			keyWithColor := fmt.Sprintf("$(RESET)%s", key)
			colorized[keyWithColor] = []map[string]any{}
			lk := listKeys[fmt.Sprintf("%s.%s", prefix, key)]
			for _, tv := range typedValue {
				child := map[string]any{}
				colorized[keyWithColor] = append(colorized[keyWithColor].([]map[string]any), child)

				pe, err := findFirst(lk, func(pe fieldpath.PathElement) bool { return matchPathElement(pe, tv.(map[string]any)) })
				if err != nil {
					return err
				}

				recursiveColorize(tv.(map[string]any), child, fmt.Sprintf("%s.%s%s", prefix, key, pe.String()))
			}
		default:
			keyWithColor := fmt.Sprintf("$(RED)%s", key)
			colorized[keyWithColor] = typedValue
		}
	}
	return nil
}

func matchPathElement(pe fieldpath.PathElement, value map[string]any) bool {
	for _, k := range *pe.Key {
		if k.Value.IsString() && value[k.Name] != k.Value.AsString() {
			return false
		}
		if k.Value.IsInt() && value[k.Name] != k.Value.AsInt() {
			return false
		}
	}
	return true
}

func findFirst[T any](s []T, f func(T) bool) (T, error) {
	for _, e := range s {
		fmt.Printf("%#v\n", e)
		if f(e) {
			return e, nil
		}
	}

	var zero T
	return zero, errors.New("not found")
}
