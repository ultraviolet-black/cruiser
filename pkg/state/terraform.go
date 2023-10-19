package state

import (
	"context"
	"encoding/json"
)

type TfstateResource struct {
	Type      string            `json:"type"`
	Name      string            `json:"name"`
	Mode      string            `json:"mode"`
	Provider  string            `json:"provider"`
	Instances []json.RawMessage `json:"instances"`
}

type Tfstate struct {
	Resources []*TfstateResource `json:"resources"`
}

type TfstateSource interface {
	GetTfstate(context.Context) ([]*Tfstate, error)
}
