package cmd

import (
	"bytes"
	"fmt"

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
	listPathElements := extractListPathElements(*allFields)

	marker := ColorMarker{
		fieldColors:      fieldColors,
		listPathElements: listPathElements,
	}

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

func extractListPathElements(fs fieldpath.Set) map[string][]fieldpath.PathElement {
	listPathElements := map[string][]fieldpath.PathElement{}
	fs.Iterate(func(p fieldpath.Path) {
		last := p[len(p)-1]
		if last.FieldName != nil || len(p) < 2 {
			return
		}

		prefix := p[0 : len(p)-1].String()
		if _, ok := listPathElements[prefix]; !ok {
			listPathElements[prefix] = []fieldpath.PathElement{}
		}
		listPathElements[prefix] = append(listPathElements[prefix], last)
	})

	return listPathElements
}

type ColorMarker struct {
	fieldColors      map[string]color
	listPathElements map[string][]fieldpath.PathElement
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

			lpe, ok := m.listPathElements[fieldPath]
			if !ok {
				marked[markedKey] = typedValue
				break
			}

			markedList := []any{}
			for i, v := range typedValue {
				switch tv := v.(type) {
				case map[string]any:
					prefix, ok := findFirst(lpe, func(pe fieldpath.PathElement) bool {
						return m.matchPathElement(pe, tv) ||
							(pe.Index != nil && *pe.Index == i)
					})
					if !ok {
						markedList = append(markedList, tv)
						break
					}
					markedChild, err := m.mark(tv, fmt.Sprintf("%s%s", fieldPath, prefix.String()))
					if err != nil {
						return nil, err
					}
					markedList = append(markedList, markedChild)
				case string:
					prefix, ok := findFirst(lpe, func(pe fieldpath.PathElement) bool {
						return (pe.Value != nil && tv == (*pe.Value).AsString()) ||
							(pe.Index != nil && *pe.Index == i)
					})
					if !ok {
						markedList = append(markedList, tv)
						break
					}
					markedList = append(markedList, m.markValue(fmt.Sprintf("%s%s", fieldPath, prefix.String()), tv))
				default:
					markedList = append(markedList, tv)
				}
			}
			marked[markedKey] = markedList
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

func (m *ColorMarker) markValue(fieldPath, value string) string {
	if c, ok := m.fieldColors[fieldPath]; ok {
		return fmt.Sprintf("%s%s%d%s", value, markerPrefix, c, markerSuffix)
	}
	return value
}

func (m *ColorMarker) matchPathElement(pe fieldpath.PathElement, value map[string]any) bool {
	if pe.Key == nil {
		return false
	}

	for _, k := range *pe.Key {
		if k.Value.IsString() && value[k.Name] == k.Value.AsString() {
			continue
		}
		if k.Value.IsInt() && value[k.Name] == k.Value.AsInt() {
			continue
		}
		if k.Value.IsFloat() && value[k.Name] != k.Value.AsFloat() {
			continue
		}
		if k.Value.IsBool() && value[k.Name] != k.Value.AsBool() {
			continue
		}
		return false
	}

	return true
}

func findFirst[T any](s []T, f func(T) bool) (T, bool) {
	for _, e := range s {
		if f(e) {
			return e, true
		}
	}

	var zero T
	return zero, false
}
