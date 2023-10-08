package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strings"
	"sync/atomic"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"sigs.k8s.io/yaml"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	quotedColorMarkRegexp = regexp.MustCompile(fmt.Sprintf("\"(.+)%s(\\d+)%s\"", markerPrefix, markerSuffix))
	colorMarkRegexp       = regexp.MustCompile(fmt.Sprintf("(.+)%s(\\d+)%s", markerPrefix, markerSuffix))
)

func colorString(str string, color color) string {
	return fmt.Sprintf("%s%dm%s%s", xterm256FgPrefix, color, str, reset)
}

type PrintFlags struct {
	JSONYamlPrintFlags *genericclioptions.JSONYamlPrintFlags

	NoDescription *bool
	OutputFormat  *string
}

func (f *PrintFlags) AllowedFormats() []string {
	formats := f.JSONYamlPrintFlags.AllowedFormats()

	return formats
}

func (f *PrintFlags) ToPrinter() (printers.ResourcePrinter, error) {
	outputFormat := ""
	if f.OutputFormat != nil {
		outputFormat = *f.OutputFormat
	}

	var printer printers.ResourcePrinter

	outputFormat = strings.ToLower(outputFormat)
	switch outputFormat {
	case "json":
		printer = &ColorJSONPrinter{}
	case "yaml":
		printer = &ColorYAMLPrinter{}
	default:
		return nil, genericclioptions.NoCompatiblePrinterError{OutputFormat: &outputFormat, AllowedFormats: f.AllowedFormats()}
	}

	if !f.JSONYamlPrintFlags.ShowManagedFields {
		printer = &printers.OmitManagedFieldsPrinter{Delegate: printer}
	}
	return printer, nil
}

func (f *PrintFlags) AddFlags(cmd *cobra.Command) {
	f.JSONYamlPrintFlags.AddFlags(cmd)

	if f.OutputFormat != nil {
		cmd.Flags().StringVarP(f.OutputFormat, "output", "o", *f.OutputFormat, fmt.Sprintf(`Output format. One of: (%s) (default yaml).`, strings.Join(f.AllowedFormats(), ", ")))
	}
	if f.NoDescription != nil {
		cmd.Flags().BoolVar(f.NoDescription, "no-color-description", *f.NoDescription, "If true, do not print description for each field color (default print description).")
	}
}

func NewPrintFlags() *PrintFlags {
	outputFormat := "yaml"
	noDescription := false

	return &PrintFlags{
		OutputFormat:  &outputFormat,
		NoDescription: &noDescription,

		JSONYamlPrintFlags: genericclioptions.NewJSONYamlPrintFlags(),
	}
}

type ColorJSONPrinter struct{}

func (p *ColorJSONPrinter) PrintObj(obj runtime.Object, w io.Writer) error {
	if printers.InternalObjectPreventer.IsForbidden(reflect.Indirect(reflect.ValueOf(obj)).Type().PkgPath()) {
		return fmt.Errorf(printers.InternalObjectPrinterErr)
	}

	if obj.GetObjectKind().GroupVersionKind().Empty() {
		return fmt.Errorf("missing apiVersion or kind; try GetObjectKind().SetGroupVersionKind() if you know the type")
	}

	var b bytes.Buffer
	encoder := json.NewEncoder(&b)
	encoder.SetIndent("", "    ")
	if err := encoder.Encode(obj); err != nil {
		return err
	}
	output := quotedColorMarkRegexp.ReplaceAll(b.Bytes(), []byte(fmt.Sprintf("%s${2}m\"${1}\"%s", xterm256FgPrefix, reset)))

	_, err := w.Write(output)
	return err
}

type ColorYAMLPrinter struct {
	printCount int64
}

func (p *ColorYAMLPrinter) PrintObj(obj runtime.Object, w io.Writer) error {
	if printers.InternalObjectPreventer.IsForbidden(reflect.Indirect(reflect.ValueOf(obj)).Type().PkgPath()) {
		return fmt.Errorf(printers.InternalObjectPrinterErr)
	}

	count := atomic.AddInt64(&p.printCount, 1)
	if count > 1 {
		if _, err := w.Write([]byte("---\n")); err != nil {
			return err
		}
	}

	if obj.GetObjectKind().GroupVersionKind().Empty() {
		return fmt.Errorf("missing apiVersion or kind; try GetObjectKind().SetGroupVersionKind() if you know the type")
	}

	data, err := yaml.Marshal(obj)
	if err != nil {
		return err
	}
	output := colorMarkRegexp.ReplaceAll(data, []byte(fmt.Sprintf("%s${2}m${1}%s", xterm256FgPrefix, reset)))

	_, err = w.Write(output)
	return err
}
