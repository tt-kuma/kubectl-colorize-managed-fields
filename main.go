package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"unsafe"

	jsoniter "github.com/json-iterator/go"
	"github.com/modern-go/reflect2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
)

type ColorizedObject struct {
	color  string
	member any
}

type ColorizedKey struct {
	Color string
	Key   string
}

func (c *ColorizedKey) String() string {
	return fmt.Sprintf("(%s)%s", c.Color, c.Key)
}

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

		fieldset.Iterate(func(p fieldpath.Path) {
			fmt.Println(p.String())
			lastPathElement := p[len(p)-1]
			if lastPathElement.FieldName != nil || len(p) < 2 {
				return
			}

			// fmt.Printf("append: %s\n", p.String())
			pe := p[0 : len(p)-1]

			fmt.Printf("append: %s: %s\n", pe.String(), p.String())

			if _, ok := listKeys[pe.String()]; !ok {
				listKeys[pe.String()] = []fieldpath.PathElement{}
			}
			listKeys[pe.String()] = append(listKeys[pe.String()], lastPathElement)
		})
	}

	for k, v := range listKeys {
		fmt.Printf("-----------%s: %#v\n", k, v)
	}

	// d, err := typed.DeducedParseableType.FromUnstructured(resource.Object)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// resource.SetManagedFields(nil)
	// b, _ := resource.MarshalJSON()
	// fmt.Println(string(b))
	// fs := &fieldpath.Set{}
	// err = fs.FromJSON(bytes.NewReader(b))
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// js, err := resource.MarshalJSON()
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// fs := &fieldpath.Set{}
	// err = fs.FromJSON(bytes.NewReader(js))
	// if err != nil {
	// 	fmt.Println("hoge")
	// 	log.Fatal(err)
	// }
	// x := d.ExtractItems(fieldset.Leaves()).AsValue().Unstructured()

	// colorized := map[any]any{}
	// fsAll.Leaves().Iterate(func(p fieldpath.Path) {
	// 	var parent, child, colorizedParent, colorizedChild any
	// 	parent = resource.Object
	// 	colorizedParent = colorized
	// 	// fmt.Printf("colorParent: %#v\n", colorizedParent)
	// 	for i, pe := range p {
	// 		fmt.Printf("%s\n", pe.String())
	// 		switch typedParent := parent.(type) {
	// 		case map[string]any:
	// 			child = typedParent[*pe.FieldName]

	// 			if i == len(p)-1 {
	// 				ck := ColorizedKey{
	// 					Color: "RESET",
	// 					Key:   *pe.FieldName,
	// 				}
	// 				colorizedParent.(map[any]any)[ck] = child
	// 			}

	// 			switch typedChild := child.(type) {
	// 			case map[string]any:
	// 				var ok bool
	// 				ck := ColorizedKey{
	// 					Color: "RESET",
	// 					Key:   *pe.FieldName,
	// 				}
	// 				colorizedChild, ok = colorizedParent.(map[any]any)[ck].(map[any]any)
	// 				if !ok {
	// 					colorizedChild = map[any]any{}
	// 					colorizedParent.(map[any]any)[ck] = colorizedChild
	// 				}
	// 			case []any:
	// 				// fmt.Printf("map: []any: %#v\n", pe)
	// 				var ok bool
	// 				ck := ColorizedKey{
	// 					Color: "RESET",
	// 					Key:   *pe.FieldName,
	// 				}
	// 				colorizedChild, ok = colorizedParent.(map[any]any)[ck].([]any)
	// 				if !ok {
	// 					colorizedChild = []any{}
	// 					colorizedParent.(map[any]any)[ck] = colorizedChild
	// 					fmt.Printf("map: []any: =====================%s=================\n", *pe.FieldName)
	// 				}
	// 				// fmt.Printf("map: []any: parent: %#v\n", parent)
	// 				// fmt.Printf("map: []any: colorParent: %#v\n", colorizedParent)
	// 				// fmt.Printf("map: []any: child: %#v\n", child)
	// 				fmt.Printf("map: []any: colorChild: %#v\n", colorizedChild)
	// 				// flag := true
	// 				// fmt.Println(p.String())
	// 				// fmt.Printf("==========%#v\n", child)
	// 				// for _, cp := range colorizedParent {
	// 				// 	fmt.Printf("==========%#v\n", cp)
	// 				// 	if pe.Key != nil {
	// 				// 		flag = true
	// 				// 		for _, k := range *pe.Key {
	// 				// 			if v, ok := cp.(map[any]any)[fmt.Sprintf("k:%s", k.Name)]; !ok || v != value.ToString(k.Value) {
	// 				// 				flag = false
	// 				// 				break
	// 				// 			}
	// 				// 		}
	// 				// 		if flag {
	// 				// 			colorizedChild = cp.(map[any]any)
	// 				// 			break
	// 				// 		}
	// 				// 	}
	// 				// 	if pe.Value != nil {
	// 				// 	}

	// 				// 	if pe.Index != nil {
	// 				// 	}

	// 				// }

	// 				// if !flag {
	// 				// 	colorizedChild = map[any]any{}
	// 				// 	for _, k := range *pe.Key {
	// 				// 		colorizedChild[fmt.Sprintf("k:%s", k.Name)] = value.ToString(k.Value)
	// 				// 	}
	// 				// }
	// 			default:
	// 				ck := ColorizedKey{
	// 					Color: "RESET",
	// 					Key:   *pe.FieldName,
	// 				}
	// 				colorizedParent.(map[any]any)[ck] = typedChild
	// 			}

	// 		case []any:
	// 			var found bool
	// 			for _, tp := range typedParent {

	// 				if pe.Key != nil {
	// 					found = true
	// 					for _, k := range *pe.Key {
	// 						fmt.Printf("%v == %v\n", tp.(map[string]any)[k.Name], k.Value.Unstructured())
	// 						// uqs, _ := strconv.Unquote(value.ToString(k.Value))
	// 						if k.Value.IsString() && tp.(map[string]any)[k.Name] != k.Value.AsString() {
	// 							found = false
	// 							break
	// 						}
	// 						if k.Value.IsInt() && tp.(map[string]any)[k.Name] != k.Value.AsInt() {
	// 							found = false
	// 							break
	// 						}
	// 					}
	// 					if found {
	// 						child = tp
	// 						break
	// 					}
	// 					// case pe.Value != nil:
	// 					// case pe.Index != nil:
	// 				}
	// 			}
	// 			found = false
	// 			for _, tp := range colorizedParent.([]any) {
	// 				if pe.Key != nil {
	// 					found = true
	// 					for _, k := range *pe.Key {
	// 						// if tp.(map[string]any)[fmt.Sprintf("k:%s", k.Name)] != value.ToString(k.Value) {
	// 						// 	found = false
	// 						// 	break
	// 						// }
	// 						if k.Value.IsString() && tp.(map[string]any)[fmt.Sprintf("k:%s", k.Name)] != k.Value.AsString() {
	// 							fmt.Println("=========================================================================================================")
	// 							found = false
	// 							break
	// 						}
	// 						if k.Value.IsInt() && tp.(map[string]any)[fmt.Sprintf("k:%s", k.Name)] != k.Value.AsInt() {
	// 							found = false
	// 							break
	// 						}
	// 					}
	// 					if found {
	// 						colorizedChild = tp
	// 						break
	// 					}
	// 					// case pe.Value != nil:
	// 					// case pe.Index != nil:
	// 				}
	// 			}
	// 			if !found {
	// 				colorizedChild = map[any]any{}
	// 				// fmt.Println("=========================================================================================================")
	// 				for _, k := range *pe.Key {
	// 					// uqs, _ := strconv.Unquote(value.ToString(k.Value))
	// 					if k.Value.IsString() {
	// 						colorizedChild.(map[any]any)[fmt.Sprintf("k:%s", k.Name)] = k.Value.AsString()
	// 					}
	// 					if k.Value.IsInt() {
	// 						colorizedChild.(map[any]any)[fmt.Sprintf("k:%s", k.Name)] = k.Value.AsInt()
	// 					}
	// 				}
	// 				// TODO: fix append
	// 				colorizedParent = append(colorizedParent.([]any), colorizedChild)
	// 			}
	// 			fmt.Printf("Parent: %#v\n", parent)
	// 			fmt.Printf("Color Parent: %#v\n", colorizedParent)
	// 		}

	// 		// if i == len(p)-1 {
	// 		// 	ck := ColorizedKey{
	// 		// 		Color: "RESET",
	// 		// 		Key:   *pe.FieldName,
	// 		// 	}
	// 		// 	colorizedParent[ck] = child
	// 		// }

	// 		colorizedParent = colorizedChild
	// 		parent = child
	// 	}
	// })

	// fmt.Printf("%#v\n", colorized)
	// jsoniter.RegisterExtension(&sampleExtension{})
	// c := jsoniter.Config{DisallowUnknownFields: false}.Froze()

	// cfg := jsoniter.Config{}.Froze()
	// cfg.RegisterExtension(&sampleExtension{})
	// s, _ := cfg.MarshalIndent(colorized, "", "  ")

	// s, _ := jsoniter.MarshalToString(map[ColorizedKey]any{
	// 	{
	// 		Color: "A",
	// 		Key:   "aaaaa",
	// 	}: "hoge",
	// })
	// fmt.Println(string(s))
	// b, _ := json.Marshal(colorized)

	// j, err := json.MarshalIndent(s, "", "  ")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// fmt.Println(string(j))
	// fmt.Printf("%v=%v", "hoge", 1)
	// fmt.Println(listKeys)
	resource.SetManagedFields(nil)
	colorized := colorizeFields(resource.Object)

	j, err := json.MarshalIndent(colorized, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(j))

	// c := strings.ReplaceAll(string(j), "\"$(RED)", "\033[31m\"")
	// c = strings.ReplaceAll(c, "\"$(RESET)", "\033[00m\"")
	// c = strings.ReplaceAll(c, " }", "\033[00m }")
	// fmt.Println(c)

}

func colorizeFields(fields map[string]any) map[string]any {
	colorized := map[string]any{}
	recursiveColorize(fields, colorized, "")

	return colorized
}

func recursiveColorize(fields, colorized map[string]any, prefix string) {
	fmt.Println(prefix)
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
			fmt.Printf("%s\n", key)

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

type wrapCodec struct {
	encodeFunc  jsoniter.EncoderFunc
	isEmptyFunc func(ptr unsafe.Pointer) bool
	decodeFunc  func(ptr unsafe.Pointer, iter *jsoniter.Iterator)
}

func (codec *wrapCodec) Encode(ptr unsafe.Pointer, stream *jsoniter.Stream) {
	codec.encodeFunc(ptr, stream)
}

func (codec *wrapCodec) IsEmpty(ptr unsafe.Pointer) bool {
	if codec.isEmptyFunc == nil {
		return false
	}

	return codec.isEmptyFunc(ptr)
}

func (codec *wrapCodec) Decode(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
	codec.decodeFunc(ptr, iter)
}

type sampleExtension struct {
	jsoniter.EncoderExtension
}

// func (e *sampleExtension) CreateMapKeyDecoder(typ reflect2.Type) jsoniter.ValDecoder {
// 	fmt.Println("===============hogehoge")

// 	if typ == reflect2.TypeOf(ColorizedKey{}) {
// 		return &wrapCodec{
// 			decodeFunc: func(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
// 				k := iter.Read()
// 				*(*string)(ptr) = k.(*ColorizedKey).String()
// 			},
// 		}
// 	}

// 	return nil
// }

func (e *sampleExtension) CreateMapKeyEncoder(typ reflect2.Type) jsoniter.ValEncoder {
	// fmt.Println(typ.String())
	// fmt.Println("===============hogehoge")
	if typ == reflect2.TypeOf(ColorizedKey{}) {
		return &wrapCodec{
			encodeFunc: func(ptr unsafe.Pointer, stream *jsoniter.Stream) {
				stream.WriteString((*ColorizedKey)(ptr).String())
			},
			isEmptyFunc: func(ptr unsafe.Pointer) bool { return false },
		}
	}

	return nil
}

// func (e *sampleExtension) CreateDecoder(typ reflect2.Type) jsoniter.ValDecoder {
// 	fmt.Println("hogehoge")

// 	if typ == reflect2.TypeOf(ColorizedKey{}) {
// 		return &wrapCodec{
// 			decodeFunc: func(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
// 				k := iter.Read()
// 				*(*string)(ptr) = k.(*ColorizedKey).String()
// 			},
// 		}
// 	}

// 	return nil
// }

// func (e *sampleExtension) CreateEncoder(typ reflect2.Type) jsoniter.ValEncoder {
// 	fmt.Println(typ.String())
// 	// if typ == reflect2.TypeOf(ColorizedKey{}) {
// 	if typ.Kind() == reflect.String {
// 		// fmt.Println("hoge")
// 		return &wrapCodec{
// 			encodeFunc: func(ptr unsafe.Pointer, stream *jsoniter.Stream) {
// 				stream.WriteString("----")
// 			},
// 			isEmptyFunc: nil,
// 		}
// 		// }
// 	}
// 	if typ == reflect2.TypeOf(map[ColorizedKey]any{}) {

// 		return &wrapCodec{
// 			encodeFunc: func(ptr unsafe.Pointer, stream *jsoniter.Stream) {
// 				stream.WriteString("=====")
// 			},
// 			isEmptyFunc: nil,
// 		}
// 		// }
// 	}
// 	return nil
// }
