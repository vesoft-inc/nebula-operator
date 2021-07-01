/*
Copyright 2021 Vesoft Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package list

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	cmdutil "github.com/vesoft-inc/nebula-operator/pkg/ngctl/cmd/util"
)

const (
	listLong = `
		List nebula clusters.

		Prints a table of the most important information about the nebula clusters. You can 
		filter the list using a label selector and the --selector flag. You will only see 
		results in your current namespace unless you pass --all-namespaces.
		
		By specifying the output as 'template' and providing a Go template as the value of 
		the --template flag, you can filter the attributes of the nebula clusters.
`
	listExample = `
		# List all nebula clusters.
		ngctl list
		
		# List all nebula clusters in all namespaces.
		ngctl list -A

		# List all nebula clusters with json format.
		ngctl list -o json

		# List a single nebula clusters with specified NAME in ps output format.
		ngctl list NAME
  
	  	# Return only the metad's phase value of the specified pod.
	  	ngctl list -o template --template="{{.status.graphd.phase}}" NAME
  
  		# List image information in custom columns.
  		ngctl list -o custom-columns=NAMESPACE:.metadata.namespace,NAME:.metadata.name,IMAGE:.spec.graphd.image
`
	OutputFormatWide = "wide"
)

type (
	// Options is a struct to support version command
	Options struct {
		LabelSelector          string
		FieldSelector          string
		AllNamespaces          bool
		Namespace              string
		NebulaClusterNames     []string
		Sort                   bool
		SortBy                 string
		IgnoreNotFound         bool
		IsHumanReadablePrinter bool
		ServerPrint            bool

		PrintFlags *PrintFlags
		ToPrinter  func(mapping *meta.RESTMapping, withNamespace bool) (printers.ResourcePrinterFunc, error)

		builder *resource.Builder
		genericclioptions.IOStreams
	}
)

// NewOptions returns initialized Options
func NewOptions(streams genericclioptions.IOStreams) *Options {
	return &Options{
		PrintFlags:  NewGetPrintFlags(),
		IOStreams:   streams,
		ServerPrint: true,
	}
}

// NewCmdList returns a cobra command for list nebula clusters
func NewCmdList(f cmdutil.Factory, ioStreams genericclioptions.IOStreams) *cobra.Command {
	o := NewOptions(ioStreams)
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "list all nebula clusters",
		Long:    listLong,
		Example: listExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args))
			cmdutil.CheckErr(o.Run())
		},
	}

	o.PrintFlags.AddFlags(cmd)
	o.AddFlags(cmd)

	return cmd
}

// AddFlags add all the flags.
func (o *Options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&o.IgnoreNotFound, "ignore-not-found", o.IgnoreNotFound,
		"If the requested object does not exist the command will return exit code 0.")
	cmd.Flags().StringVarP(&o.LabelSelector, "selector", "l", o.LabelSelector,
		"Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2)")
	cmd.Flags().StringVar(&o.FieldSelector, "field-selector", o.FieldSelector,
		"Selector (field query) to filter on, supports '=', '==', and '!='.(e.g. --field-selector key1=value1,key2=value2)."+
			"The server only supports a limited number of field queries per type.")
	cmd.Flags().BoolVarP(&o.AllNamespaces, "all-namespaces", "A", o.AllNamespaces,
		"If present, list the nebula clusters across all namespaces.")
	cmd.Flags().BoolVar(&o.ServerPrint, "server-print", o.ServerPrint,
		"If true, have the server return the appropriate table output. Supports extension APIs and CRDs.")
}

// Complete completes all the required options
func (o *Options) Complete(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
	var err error
	o.NebulaClusterNames, o.Namespace, err = f.GetNebulaClusterNamesAndNamespace(false, args)
	if err != nil && !cmdutil.IsErNotSpecified(err) {
		return err
	}

	o.SortBy, err = cmd.Flags().GetString("sort-by")
	if err != nil {
		return err
	}
	o.Sort = o.SortBy != ""

	outputOption := cmd.Flags().Lookup("output").Value.String()
	if outputOption == "custom-columns" {
		o.ServerPrint = false
	}

	templateArg := ""
	if o.PrintFlags.TemplateFlags != nil && o.PrintFlags.TemplateFlags.TemplateArgument != nil {
		templateArg = *o.PrintFlags.TemplateFlags.TemplateArgument
	}

	if (*o.PrintFlags.OutputFormat == "" && templateArg == "") || *o.PrintFlags.OutputFormat == OutputFormatWide {
		o.IsHumanReadablePrinter = true
	}

	o.ToPrinter = func(mapping *meta.RESTMapping, withNamespace bool) (printers.ResourcePrinterFunc, error) {
		printFlags := o.PrintFlags.Copy()

		if mapping != nil {
			printFlags.SetKind(mapping.GroupVersionKind.GroupKind())
		}
		if withNamespace {
			if err := printFlags.EnsureWithNamespace(); err != nil {
				return nil, err
			}
		}

		printer, err := printFlags.ToPrinter()
		if err != nil {
			return nil, err
		}
		printer, err = printers.NewTypeSetter(scheme.Scheme).WrapToPrinter(printer, nil)
		if err != nil {
			return nil, err
		}

		if o.Sort {
			printer = &SortingPrinter{Delegate: printer, SortField: o.SortBy}
		}

		if o.ServerPrint {
			printer = &TablePrinter{Delegate: printer}
		}

		return printer.PrintObj, nil
	}

	o.builder = f.NewBuilder()

	return nil
}

func (o *Options) Validate(cmd *cobra.Command) error {
	if showLabels, err := cmd.Flags().GetBool("show-labels"); err != nil {
		return err
	} else if showLabels {
		outputOption := cmd.Flags().Lookup("output").Value.String()
		if outputOption != "" && outputOption != OutputFormatWide {
			return fmt.Errorf("--show-labels option cannot be used with %s printer", outputOption)
		}
	}

	return nil
}

func (o *Options) transformRequests(req *rest.Request) {
	if !o.ServerPrint || !o.IsHumanReadablePrinter {
		return
	}

	req.SetHeader("Accept", strings.Join([]string{
		fmt.Sprintf("application/json;as=Table;v=%s;g=%s", metav1.SchemeGroupVersion.Version, metav1.GroupName),
		fmt.Sprintf("application/json;as=Table;v=%s;g=%s", metav1beta1.SchemeGroupVersion.Version, metav1beta1.GroupName),
		"application/json",
	}, ","))

	if o.Sort {
		req.Param("includeObject", "Object")
	}
}

// Run executes use command
func (o *Options) Run() error {
	r := o.builder.
		Unstructured().
		NamespaceParam(o.Namespace).DefaultNamespace().AllNamespaces(o.AllNamespaces).
		LabelSelectorParam(o.LabelSelector).
		FieldSelectorParam(o.FieldSelector).
		ResourceTypeOrNameArgs(true, append([]string{"nebulacluster"}, o.NebulaClusterNames...)...).
		ContinueOnError().
		Latest().
		Flatten().
		TransformRequests(o.transformRequests).
		Do()

	if o.IgnoreNotFound {
		r.IgnoreErrors(apierrors.IsNotFound)
	}
	if err := r.Err(); err != nil {
		return err
	}

	if !o.IsHumanReadablePrinter {
		return o.printGeneric(r)
	}

	infos, err := r.Infos()
	if err != nil {
		return err
	}

	objs := make([]runtime.Object, len(infos))
	for ix := range infos {
		objs[ix] = infos[ix].Object
	}

	var positioner OriginalPositioner
	if o.Sort {
		sorter := NewRuntimeSorter(objs, o.SortBy)
		if err := sorter.Sort(); err != nil {
			return err
		}
		positioner = sorter
	}

	w := printers.GetNewTabWriter(o.Out)
	defer func() {
		_ = w.Flush()
	}()

	var printer printers.ResourcePrinter
	for ix := range objs {
		var mapping *meta.RESTMapping
		var info *resource.Info

		if positioner != nil {
			info = infos[positioner.OriginalPosition(ix)]
			mapping = info.Mapping
		} else {
			info = infos[ix]
			mapping = info.Mapping
		}

		printWithNamespace := o.AllNamespaces
		if mapping != nil && mapping.Scope.Name() == meta.RESTScopeNameRoot {
			printWithNamespace = false
		}

		if printer == nil {
			printer, err = o.ToPrinter(mapping, printWithNamespace)
			if err != nil {
				return err
			}
		}

		if err := printer.PrintObj(info.Object, w); err != nil {
			return err
		}
	}

	return nil
}

func (o *Options) printGeneric(r *resource.Result) error {
	var errs []error
	singleItemImplied := false
	infos, err := r.IntoSingleItemImplied(&singleItemImplied).Infos()
	if err != nil {
		if singleItemImplied {
			return err
		}
		errs = append(errs, err)
	}

	if len(infos) == 0 && o.IgnoreNotFound {
		return utilerrors.Reduce(utilerrors.Flatten(utilerrors.NewAggregate(errs)))
	}

	printer, err := o.ToPrinter(nil, false)
	if err != nil {
		return err
	}

	var obj runtime.Object
	if !singleItemImplied || len(infos) != 1 {
		list := corev1.List{
			TypeMeta: metav1.TypeMeta{
				Kind:       "List",
				APIVersion: "v1",
			},
			ListMeta: metav1.ListMeta{},
		}

		for _, info := range infos {
			list.Items = append(list.Items, runtime.RawExtension{Object: info.Object})
		}

		listData, err := json.Marshal(list)
		if err != nil {
			return err
		}

		converted, err := runtime.Decode(unstructured.UnstructuredJSONScheme, listData)
		if err != nil {
			return err
		}

		obj = converted
	} else {
		obj = infos[0].Object
	}

	if !meta.IsListType(obj) {
		return printer.PrintObj(obj, o.Out)
	}

	items, err := meta.ExtractList(obj)
	if err != nil {
		return err
	}

	list := &unstructured.UnstructuredList{
		Object: map[string]interface{}{
			"kind":       "List",
			"apiVersion": "v1",
			"metadata":   map[string]interface{}{},
		},
	}
	if listMeta, err := meta.ListAccessor(obj); err == nil {
		list.Object["metadata"] = map[string]interface{}{
			"selfLink":        listMeta.GetSelfLink(),
			"resourceVersion": listMeta.GetResourceVersion(),
		}
	}

	for _, item := range items {
		list.Items = append(list.Items, *item.(*unstructured.Unstructured))
	}
	return printer.PrintObj(list, o.Out)
}
