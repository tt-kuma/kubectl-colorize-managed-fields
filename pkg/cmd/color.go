package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
)

type color int

const (
	conflicted color = 1
	base       color = 2
)

const (
	escape           = "\x1b"
	xterm256FgPrefix = escape + "[38;5;"
	reset            = escape + "[0m"
	markerPrefix     = "__"
	markerSuffix     = "__"
)

var (
	colorMarkRegexp = regexp.MustCompile(fmt.Sprintf("\"(.+)%s(\\d+)%s\"", markerPrefix, markerSuffix))
)

func markWithColor(resource *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	fieldManagers := map[string][]string{}
	managerColors := make(map[string]color, len(resource.GetManagedFields()))
	allFields := &fieldpath.Set{}
	for i, mf := range resource.GetManagedFields() {
		fs := &fieldpath.Set{}
		if err := fs.FromJSON(bytes.NewReader(mf.FieldsV1.Raw)); err != nil {
			return nil, err
		}

		fs.Leaves().Iterate(func(p fieldpath.Path) {
			ps := p.String()
			if _, ok := fieldManagers[ps]; !ok {
				fieldManagers[ps] = []string{}
			}
			fieldManagers[ps] = append(fieldManagers[ps], mf.Manager)
		})

		managerColors[mf.Manager] = base + color(i)

		allFields = allFields.Union(fs)
	}

	fieldColors := assignColorToFields(fieldManagers, managerColors)
	kpe := getKeyPathElements(*allFields)

	marker := ColorMarker{
		fieldColors:     fieldColors,
		keyPathElements: kpe,
	}
	resource.SetManagedFields(nil)
	marked, err := marker.mark(resource.Object, "")
	if err != nil {
		return nil, err
	}

	return &unstructured.Unstructured{
		Object: marked,
	}, nil
}

func assignColorToFields(fieldManagers map[string][]string, managerColors map[string]color) map[string]color {
	fieldColors := map[string]color{}
	for k, v := range fieldManagers {
		if len(v) == 0 {
			continue
		}
		if len(v) > 1 {

			fieldColors[k] = conflicted
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

		prefix := p[0 : len(p)-1].String()
		if _, ok := kpe[prefix]; !ok {
			kpe[prefix] = []fieldpath.PathElement{}
		}
		kpe[prefix] = append(kpe[prefix], last)
	})

	return kpe
}

func colorJSON(j string) string {
	colorized := j
	colorized = colorMarkRegexp.ReplaceAllString(colorized, fmt.Sprintf("%s${2}m\"${1}\"%s", xterm256FgPrefix, reset))

	return colorized
}

type ColorMarker struct {
	fieldColors     map[string]color
	keyPathElements map[string][]fieldpath.PathElement
}

func (m *ColorMarker) mark(obj map[string]any, pathPrefix string) (map[string]any, error) {
	marked := map[string]any{}
	for key, value := range obj {
		markedKey := m.markKey(pathPrefix, key)
		fieldPath := fmt.Sprintf("%s.%s", pathPrefix, key)

		switch typedValue := value.(type) {
		case map[string]any:
			markedChild, err := m.mark(typedValue, fieldPath)
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
			lk := m.keyPathElements[fieldPath]
			for _, tv := range typedValue {
				var prefix fieldpath.PathElement
				if len(lk) != 0 {
					var err error
					prefix, err = findFirst(lk, func(prefix fieldpath.PathElement) bool {
						return m.matchPathElement(prefix, tv.(map[string]any))
					})
					if err != nil {
						return nil, err
					}
				}

				markedChild, err := m.mark(tv.(map[string]any), fmt.Sprintf("%s%s", fieldPath, prefix.String()))
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

func (m *ColorMarker) markKey(pathPrefix, key string) string {
	if c, ok := m.fieldColors[fmt.Sprintf("%s.%s", pathPrefix, key)]; ok {
		return fmt.Sprintf("%s%s%d%s", key, markerPrefix, c, markerSuffix)
	}
	return key
}

// TODO: support other types
func (m *ColorMarker) matchPathElement(prefix fieldpath.PathElement, value map[string]any) bool {
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

func findFirst[T any](s []T, f func(T) bool) (T, error) {
	for _, e := range s {
		if f(e) {
			return e, nil
		}
	}

	var zero T
	return zero, errors.New("not found")
}
