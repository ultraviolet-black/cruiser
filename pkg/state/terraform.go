package state

import (
	"context"
)

type TfstateResourceInstance struct {
	ProtoJson string `json:"proto_json"`
}

type TfstateResource struct {
	Type      string                     `json:"type"`
	Name      string                     `json:"name"`
	Mode      string                     `json:"mode"`
	Provider  string                     `json:"provider"`
	Instances []*TfstateResourceInstance `json:"instances"`
}

type Tfstate struct {
	Resources []*TfstateResource `json:"resources"`
}

type TfstateSource interface {
	GetTfstate(context.Context) ([]*Tfstate, error)
}
