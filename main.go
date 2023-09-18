package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
)

type Color int

const (
	Reset Color = 0
)

const (
	Red Color = iota + 31
	Green
	Yellow
	Blue
	Magenta
	Cyan
)

var (
	colorMarkRegexp = regexp.MustCompile("\"(.+)__(\\d+)__\"")
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

	fieldManagers := map[string][]string{}
	managerColors := make(map[string]Color, len(resource.GetManagedFields()))
	allFields := &fieldpath.Set{}
	for i, mf := range resource.GetManagedFields() {
		fs := &fieldpath.Set{}
		if err := fs.FromJSON(bytes.NewReader(mf.FieldsV1.Raw)); err != nil {
			log.Fatal(err)
		}

		fs.Leaves().Iterate(func(p fieldpath.Path) {
			ps := p.String()
			if _, ok := fieldManagers[ps]; !ok {
				fieldManagers[ps] = []string{}
			}
			fieldManagers[ps] = append(fieldManagers[ps], mf.Manager)
		})

		managerColors[mf.Manager] = Green + Color(i)

		allFields = allFields.Union(fs)
	}

	fieldColors := assignColorToFields(fieldManagers, managerColors)
	kpe := getKeyPathElements(*allFields)

	resource.SetManagedFields(nil)
	marked, err := markWithColor(resource.Object, "", fieldColors, kpe)
	if err != nil {
		log.Fatal(err)
	}

	j, err := json.MarshalIndent(marked, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	cj := colorizeJSON(string(j))

	fmt.Println(cj)
}

func assignColorToFields(fieldManagers map[string][]string, managerColors map[string]Color) map[string]Color {
	fieldColors := map[string]Color{}
	for k, v := range fieldManagers {
		if len(v) == 0 {
			continue
		}
		if len(v) > 1 {

			fieldColors[k] = Red
			continue
		}
		fieldColors[k] = managerColors[v[0]]
	}

	return fieldColors
}

func getKeyPathElements(fs fieldpath.Set) map[string][]fieldpath.PathElement {
	kpe := map[string][]fieldpath.PathElement{}
	fs.Iterate(func(p fieldpath.Path) {
		last := p[len(p)-1]
		if last.FieldName != nil || len(p) < 2 {
			return
		}

		prefix := p[0 : len(p)-1]
		if _, ok := kpe[prefix.String()]; !ok {
			kpe[prefix.String()] = []fieldpath.PathElement{}
		}
		kpe[prefix.String()] = append(kpe[prefix.String()], last)
	})

	return kpe
}

func markWithColor(obj map[string]any, pathPrefix string, colors map[string]Color, kpe map[string][]fieldpath.PathElement) (map[string]any, error) {
	marked := map[string]any{}
	for key, value := range obj {
		markedKey := markKeyWithColor(pathPrefix, key, colors)
		fieldPath := fmt.Sprintf("%s.%s", pathPrefix, key)

		switch typedValue := value.(type) {
		case map[string]any:
			markedChild, err := markWithColor(typedValue, fieldPath, colors, kpe)
			if err != nil {
				return nil, err
			}
			marked[markedKey] = markedChild
		case []any:
			if len(typedValue) == 0 {
				marked[markedKey] = typedValue
				break
			}

			if _, ok := typedValue[0].(map[string]any); !ok {
				marked[markedKey] = typedValue
				break
			}

			marked[markedKey] = []map[string]any{}
			lk := kpe[fieldPath]
			for _, tv := range typedValue {
				prefix, err := findFirst(lk, func(prefix fieldpath.PathElement) bool { return matchPathElement(prefix, tv.(map[string]any)) })
				if err != nil {
					return nil, fmt.Errorf("failed to get matched path element: %w", err)
				}
				markedChild, err := markWithColor(tv.(map[string]any), fmt.Sprintf("%s%s", fieldPath, prefix.String()), colors, kpe)
				if err != nil {
					return nil, err
				}
				marked[markedKey] = append(marked[markedKey].([]map[string]any), markedChild)
			}
		default:
			marked[markedKey] = typedValue
		}
	}
	return marked, nil
}

func markKeyWithColor(pathPrefix, key string, colors map[string]Color) string {
	color := Reset
	if c, ok := colors[fmt.Sprintf("%s.%s", pathPrefix, key)]; ok {
		color = c
	}
	return fmt.Sprintf("%s__%d__", key, color)
}

func matchPathElement(prefix fieldpath.PathElement, value map[string]any) bool {
	for _, k := range *prefix.Key {
		if k.Value.IsString() && value[k.Name] != k.Value.AsString() {
			return false
		}
		if k.Value.IsInt() && value[k.Name] != k.Value.AsInt() {
			return false
		}
	}
	return true
}

func colorizeJSON(j string) string {
	colorized := j
	colorized = colorMarkRegexp.ReplaceAllString(colorized, "\033[${2}m\"${1}\"")
	colorized = strings.ReplaceAll(colorized, " }", "\033[00m }")

	return colorized
}

func findFirst[T any](s []T, f func(T) bool) (T, error) {
	for _, e := range s {
		if f(e) {
			return e, nil
		}
	}

	var zero T
	return zero, errors.New("not found")
}
