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
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/vesoft-inc/nebula-operator/apis/pkg/label"
	cmdutil "github.com/vesoft-inc/nebula-operator/pkg/ngctl/cmd/util"
)

var (
	listLong = templates.LongDesc(`
		List nebula clusters or there sub resources.

		Prints a table of the most important information about the nebula cluster resources. 
		You can use many of the same flags as kubectl get`)

	listExample = templates.Examples(`
		# List all nebula clusters.
		ngctl list
		
		# List all nebula clusters in all namespaces.
		ngctl list -A

		# List all nebula clusters with json format.
		ngctl list -o json

		# List nebula cluster sub resources with specified cluster name.
		ngctl list pod --nebulacluster=nebula
  
	  	# Return only the metad's phase value of the specified pod.
	  	ngctl list -o template --template="{{.status.graphd.phase}}" NAME
  
  		# List image information in custom columns.
  		ngctl list -o custom-columns=NAMESPACE:.metadata.namespace,NAME:.metadata.name,IMAGE:.spec.graphd.image`)
)

const (
	OutputFormatWide = "wide"
)

// ListOptions is a struct to support list command
type ListOptions struct {
	Namespace          string
	NebulaClusterName  string
	NebulaClusterLabel string
	ResourceType       string

	LabelSelector  string
	FieldSelector  string
	AllNamespaces  bool
	Sort           bool
	SortBy         string
	IgnoreNotFound bool

	ServerPrint bool

	PrintFlags             *PrintFlags
	ToPrinter              func(mapping *meta.RESTMapping, withNamespace bool) (printers.ResourcePrinterFunc, error)
	IsHumanReadablePrinter bool

	builder *resource.Builder
	genericclioptions.IOStreams
}

// NewListOptions returns initialized Options
func NewListOptions(streams genericclioptions.IOStreams) *ListOptions {
	return &ListOptions{
		PrintFlags:  NewGetPrintFlags(),
		IOStreams:   streams,
		ServerPrint: true,
	}
}

// NewCmdList returns a cobra command for list nebula clusters or there sub resources.
func NewCmdList(f cmdutil.Factory, ioStreams genericclioptions.IOStreams) *cobra.Command {
	o := NewListOptions(ioStreams)
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "list all nebula clusters",
		Long:    listLong,
		Example: listExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args))
			cmdutil.CheckErr(o.Validate(cmd))
			cmdutil.CheckErr(o.Run())
		},
	}

	o.PrintFlags.AddFlags(cmd)
	f.AddFlags(cmd)
	o.AddFlags(cmd)

	return cmd
}

// AddFlags add extra list options flags.
func (o *ListOptions) AddFlags(cmd *cobra.Command) {
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
func (o *ListOptions) Complete(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
	var err error

	o.NebulaClusterName, o.Namespace, err = f.GetNebulaClusterNameAndNamespace(false, nil)
	if err != nil && !cmdutil.IsErNotSpecified(err) {
		return err
	}

	o.NebulaClusterLabel = label.ClusterLabelKey + "=" + o.NebulaClusterName

	if len(args) > 0 {
		o.ResourceType = args[0]
	} else {
		o.ResourceType = cmdutil.NebulaClusterResourceType
		o.NebulaClusterLabel = ""
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

func (o *ListOptions) Validate(cmd *cobra.Command) error {
	if showLabels, err := cmd.Flags().GetBool("show-labels"); err != nil {
		return err
	} else if showLabels {
		outputOption := cmd.Flags().Lookup("output").Value.String()
		if outputOption != "" && outputOption != OutputFormatWide {
			return fmt.Errorf("--show-labels option cannot be used with %s printer", outputOption)
		}
	}

	if o.NebulaClusterName == "" && o.ResourceType != cmdutil.NebulaClusterResourceType {
		return cmdutil.UsageErrorf(cmd, "using '--nebulacluster' to set nebula cluster first.")
	}

	return nil
}

func (o *ListOptions) transformRequests(req *rest.Request) {
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

// Run executes list command
func (o *ListOptions) Run() error {
	if o.LabelSelector == "" {
		o.LabelSelector = o.NebulaClusterLabel
	} else {
		o.LabelSelector = o.LabelSelector + "," + o.NebulaClusterLabel
	}

	r := o.builder.
		Unstructured().
		NamespaceParam(o.Namespace).DefaultNamespace().AllNamespaces(o.AllNamespaces).
		LabelSelectorParam(o.LabelSelector).
		FieldSelectorParam(o.FieldSelector).
		ResourceTypeOrNameArgs(true, o.ResourceType).
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

func (o *ListOptions) printGeneric(r *resource.Result) error {
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
