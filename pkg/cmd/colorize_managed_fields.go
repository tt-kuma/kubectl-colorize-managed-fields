package cmd

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type ColorizeManagedFieldsOptions struct {
	ExplicitNamespace bool
	Namespace         string

	genericiooptions.IOStreams
}

func NewColorizeManagedFieldsOptions(streams genericiooptions.IOStreams) *ColorizeManagedFieldsOptions {
	return &ColorizeManagedFieldsOptions{
		IOStreams: streams,
	}
}

func NewCmdColorizeManagedFields(streams genericiooptions.IOStreams) *cobra.Command {
	o := NewColorizeManagedFieldsOptions(streams)

	defaultConfigFlags := genericclioptions.NewConfigFlags(true)
	matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(defaultConfigFlags)
	f := cmdutil.NewFactory(matchVersionKubeConfigFlags)

	cmd := &cobra.Command{
		Use:     "kubectl colorize-managed-fields",
		Short:   "",
		Long:    "",
		Example: "",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, args))
			cmdutil.CheckErr(o.Run(f, args))
		},
	}

	flags := cmd.Flags()
	matchVersionKubeConfigFlags.AddFlags(flags)

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

	allErrs := []error{}
	infos, err := r.Infos()
	if err != nil {
		allErrs = append(allErrs, err)
	}
	if len(infos) > 1 {
		allErrs = append(allErrs, errors.New("support only single resource"))
		return utilerrors.NewAggregate(allErrs)
	}

	resource := infos[0].Object.DeepCopyObject().(*unstructured.Unstructured)
	colorized, err := colorize(resource)
	if err != nil {
		allErrs = append(allErrs, err)
	}

	j, err := json.MarshalIndent(colorized.Object, "", "  ")
	if err != nil {
		allErrs = append(allErrs, err)
	}

	cj := colorizeJSON(string(j))
	fmt.Fprintln(o.IOStreams.Out, cj)

	return utilerrors.NewAggregate(allErrs)
}
