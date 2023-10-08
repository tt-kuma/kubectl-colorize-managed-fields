package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	colorizeLong = templates.LongDesc(i18n.T(`
		Display one resource with colorized fields based on managed fields.

		Fields managed by a single manager are colorized uniquely to distinguish each
		manager. Fields with more than two managers are uniformly colorized by predefined
		conflicted color, regardless of the combination of managers.
		Currently, only one resource is supported. If you specify more than two resources,
		you will receive an error.`))
	colorizeExample = templates.Examples(i18n.T(`
		# Display a single pod
		kubectl colorize-managed-fields pod sample-pod`))
)

type ColorizeManagedFieldsOptions struct {
	PrintFlags *PrintFlags

	Namespace         string
	ExplicitNamespace bool

	genericiooptions.IOStreams
}

func NewColorizeManagedFieldsOptions(streams genericiooptions.IOStreams) *ColorizeManagedFieldsOptions {
	return &ColorizeManagedFieldsOptions{
		PrintFlags: NewPrintFlags(),
		IOStreams:  streams,
	}
}

func NewCmdColorizeManagedFields(streams genericiooptions.IOStreams) *cobra.Command {
	o := NewColorizeManagedFieldsOptions(streams)

	defaultConfigFlags := genericclioptions.NewConfigFlags(true)
	matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(defaultConfigFlags)
	f := cmdutil.NewFactory(matchVersionKubeConfigFlags)

	cmd := &cobra.Command{
		Use:     "kubectl colorize-managed-fields",
		Short:   "Display one resource with colorized fields based on managed fields",
		Long:    colorizeLong,
		Example: colorizeExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, args))
			cmdutil.CheckErr(o.Run(f, args))
		},
	}

	flags := cmd.Flags()
	defaultConfigFlags.AddFlags(flags)
	o.PrintFlags.AddFlags(cmd)

	return cmd
}

func (o *ColorizeManagedFieldsOptions) Complete(f cmdutil.Factory, args []string) error {
	var err error
	o.Namespace, o.ExplicitNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	return nil
}

func (o *ColorizeManagedFieldsOptions) Run(f cmdutil.Factory, args []string) error {
	r := f.NewBuilder().
		Unstructured().
		NamespaceParam(o.Namespace).DefaultNamespace().
		ResourceTypeOrNameArgs(true, args...).
		ContinueOnError().
		Latest().
		Flatten().
		Do()

	if err := r.Err(); err != nil {
		return err
	}

	infos, err := r.Infos()
	if err != nil {
		return err
	}

	if len(infos) == 0 {
		fmt.Fprintf(o.ErrOut, "No resources found in %s namespace.\n", o.Namespace)
		return nil
	}

	if len(infos) > 1 {
		return errors.New("support only a single resource")
	}

	resource := infos[0].Object.DeepCopyObject().(*unstructured.Unstructured)
	marked, managerColors, err := markWithColor(resource)
	if err != nil || marked == nil {
		return fmt.Errorf("failed to colorize a object: %w", err)
	}

	if !*o.PrintFlags.NoDescription {
		fmt.Fprintln(o.IOStreams.Out, "COLOR"+"\t"+"MANAGER")
		for k, v := range managerColors {
			fmt.Fprintln(o.IOStreams.Out, colorString("■", v)+"\t"+k)
		}
		fmt.Fprintln(o.IOStreams.Out, colorString("■", conflicted)+"\t"+"more than two managers")
		fmt.Fprintln(o.IOStreams.Out, "===")
	}

	printer, err := o.PrintFlags.ToPrinter()
	if err != nil {
		return err
	}

	return printer.PrintObj(marked, o.IOStreams.Out)
}
