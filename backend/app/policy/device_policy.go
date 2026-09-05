package policy

import (
	"errors"
	"sort"
)

type Document struct {
	Unattended    Unattended    `json:"unattended"`
	ServerProfile ServerProfile `json:"server_profile"`
}

type Unattended struct {
	Enabled     *bool   `json:"enabled,omitempty"`
	RootCommand *string `json:"root_command,omitempty"`
}

type ServerProfile struct {
	Enabled           *bool   `json:"enabled,omitempty"`
	IDServer          *string `json:"id_server,omitempty"`
	RelayServer       *string `json:"relay_server,omitempty"`
	APIServer         *string `json:"api_server,omitempty"`
	Key               *string `json:"key,omitempty"`
	PermanentPassword *string `json:"permanent_password,omitempty"`
}

type GroupDocument struct {
	GroupID  int
	Priority int
	Document Document
}

type Effective struct {
	UnattendedEnabled bool
	RootCommand       string
	ProfileEnabled    bool
	IDServer          string
	RelayServer       string
	APIServer         string
	Key               string
	PermanentPassword string
}

func Resolve(global *Document, groups []GroupDocument, device *Document) (Effective, error) {
	effective := Effective{RootCommand: "auto"}
	if global != nil {
		apply(&effective, global)
	}
	orderedGroups := append([]GroupDocument(nil), groups...)
	sort.SliceStable(orderedGroups, func(i, j int) bool {
		if orderedGroups[i].Priority == orderedGroups[j].Priority {
			return orderedGroups[i].GroupID < orderedGroups[j].GroupID
		}
		return orderedGroups[i].Priority < orderedGroups[j].Priority
	})
	for i := range orderedGroups {
		apply(&effective, &orderedGroups[i].Document)
	}
	if device != nil {
		apply(&effective, device)
	}
	if effective.RootCommand != "auto" && effective.RootCommand != "su" && effective.RootCommand != "testsu" && effective.RootCommand != "disabled" {
		return Effective{}, errors.New("invalid root command")
	}
	if effective.ProfileEnabled && effective.IDServer == "" {
		return Effective{}, errors.New("enabled server profile requires an ID server")
	}
	return effective, nil
}

func apply(effective *Effective, document *Document) {
	if document.Unattended.Enabled != nil {
		effective.UnattendedEnabled = *document.Unattended.Enabled
	}
	if document.Unattended.RootCommand != nil {
		effective.RootCommand = *document.Unattended.RootCommand
	}
	if document.ServerProfile.Enabled != nil {
		effective.ProfileEnabled = *document.ServerProfile.Enabled
	}
	if document.ServerProfile.IDServer != nil {
		effective.IDServer = *document.ServerProfile.IDServer
	}
	if document.ServerProfile.RelayServer != nil {
		effective.RelayServer = *document.ServerProfile.RelayServer
	}
	if document.ServerProfile.APIServer != nil {
		effective.APIServer = *document.ServerProfile.APIServer
	}
	if document.ServerProfile.Key != nil {
		effective.Key = *document.ServerProfile.Key
	}
	if document.ServerProfile.PermanentPassword != nil {
		effective.PermanentPassword = *document.ServerProfile.PermanentPassword
	}
}
