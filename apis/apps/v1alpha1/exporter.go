package v1alpha1

// +k8s:deepcopy-gen=false
type NebulaExporterComponent interface {
	ComponentSpec() ComponentAccessor
	MaxRequests() int32
}

var _ NebulaExporterComponent = &exporterComponent{}

// +k8s:deepcopy-gen=false
func newExporterComponent(nc *NebulaCluster) *exporterComponent {
	return &exporterComponent{nc: nc}
}

// +k8s:deepcopy-gen=false
type exporterComponent struct {
	nc *NebulaCluster
}

func (e *exporterComponent) ComponentSpec() ComponentAccessor {
	return buildComponentAccessor(e.nc, &e.nc.Spec.Exporter.ComponentSpec)
}

func (e *exporterComponent) MaxRequests() int32 {
	return e.nc.Spec.Exporter.MaxRequests
}
