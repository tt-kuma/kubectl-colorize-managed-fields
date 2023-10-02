package cmd

import (
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

type PrintFlags struct {
	JSONYamlPrintFlags *genericclioptions.JSONYamlPrintFlags

	ShowDescription *bool
	OutputFormat    *string
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
		cmd.Flags().StringVarP(f.OutputFormat, "output", "o", *f.OutputFormat, fmt.Sprintf(`Output format. One of: (%s).`, strings.Join(f.AllowedFormats(), ", ")))
	}
	if f.ShowDescription != nil {
		cmd.Flags().BoolVar(f.ShowDescription, "show-color-description", *f.ShowDescription, "If true, print description for each field color")
	}
}

func NewPrintFlags() *PrintFlags {
	outputFormat := "yaml"
	ShowDescription := false

	return &PrintFlags{
		OutputFormat:    &outputFormat,
		ShowDescription: &ShowDescription,

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

	data, err := json.MarshalIndent(obj, "", "    ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	data = quotedColorMarkRegexp.ReplaceAll(data, []byte(fmt.Sprintf("%s${2}m\"${1}\"%s", xterm256FgPrefix, reset)))

	_, err = w.Write(data)
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

	output, err := yaml.Marshal(obj)
	if err != nil {
		return err
	}
	output = colorMarkRegexp.ReplaceAll(output, []byte(fmt.Sprintf("%s${2}m${1}%s", xterm256FgPrefix, reset)))
	_, err = fmt.Fprint(w, string(output))
	return err
}
