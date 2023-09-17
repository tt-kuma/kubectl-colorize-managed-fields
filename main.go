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
	mfs := resource.GetManagedFields()
	fsAll := &fieldpath.Set{}
	for _, mf := range mfs {
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

func recursiveColorize(fields, colorized map[string]any, prefix string) {
	for key, value := range fields {
		switch typedValue := value.(type) {
		case map[string]any:
			keyWithColor := fmt.Sprintf("$(RESET)%s", key)
			child := map[string]any{}
			colorized[keyWithColor] = child
			recursiveColorize(typedValue, child, fmt.Sprintf("%s.%s", prefix, key))
		case []any:
			_, ok := typedValue[0].(map[string]any)
			if !ok {
				fmt.Printf("%s\n", key)
				keyWithColor := fmt.Sprintf("$(RED)%s", key)
				colorized[keyWithColor] = typedValue
				break
			}

			keyWithColor := fmt.Sprintf("$(RESET)%s", key)
			var children []map[string]any
			colorized[keyWithColor] = children

			lk := listKeys[fmt.Sprintf("%s.%s", prefix, key)]
			for _, tv := range typedValue {
				child := map[string]any{}
				colorized[keyWithColor] = append(colorized[keyWithColor].([]map[string]any), child)

				var pathElement fieldpath.PathElement
				var found bool
				for _, pe := range lk {
					found = true
					fmt.Printf("%#v\n", pe)
					for _, k := range *pe.Key {
						if k.Value.IsString() && tv.(map[string]any)[k.Name] != k.Value.AsString() {
							found = false
							break
						}
						if k.Value.IsInt() && tv.(map[string]any)[k.Name] != k.Value.AsInt() {
							found = false
							break
						}
					}
					if found {
						pathElement = pe
						break
					}
				}
				if !found {
					log.Fatalln("pe not found")
				}

				recursiveColorize(tv.(map[string]any), child, fmt.Sprintf("%s.%s%s", prefix, key, pathElement.String()))
			}
		default:
			keyWithColor := fmt.Sprintf("$(RED)%s", key)
			colorized[keyWithColor] = typedValue
		}
	}
}
