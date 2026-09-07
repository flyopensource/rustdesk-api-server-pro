package policy

import (
	"errors"
	"rustdesk-api-server-pro/app/model"
	"time"

	"xorm.io/xorm"
)

func CurrentRevision(db *xorm.Engine) (int64, error) {
	state := model.StrategyState{Id: 1}
	has, err := db.ID(state.Id).Get(&state)
	if err != nil {
		return 0, err
	}
	if has {
		return state.Revision, nil
	}
	state.Revision = time.Now().UnixMilli()
	if _, err = db.Insert(&state); err != nil {
		if has, loadErr := db.ID(state.Id).Get(&state); loadErr == nil && has {
			return state.Revision, nil
		}
		return 0, err
	}
	return state.Revision, nil
}

func NextRevision(session *xorm.Session) (int64, error) {
	state := model.StrategyState{Id: 1}
	has, err := session.ID(state.Id).Get(&state)
	if err != nil {
		return 0, err
	}
	next := time.Now().UnixMilli()
	if has && next <= state.Revision {
		next = state.Revision + 1
	}
	state.Revision = next
	if has {
		_, err = session.ID(state.Id).Cols("revision").Update(&state)
	} else {
		_, err = session.Insert(&state)
	}
	return next, err
}

func ResolveForDevice(db *xorm.Engine, device *model.Device) (Effective, error) {
	revision, err := CurrentRevision(db)
	if err != nil {
		return Effective{}, err
	}
	rootCommand := device.RootCommand
	if rootCommand == "" {
		rootCommand = "auto"
	}
	effective := Effective{
		Revision:          revision,
		UnattendedEnabled: device.UnattendedEnabled,
		RootCommand:       rootCommand,
	}
	candidates := []struct {
		scope  string
		id     int
		source string
	}{{model.StrategyScopeDevice, device.Id, "device"}}
	membership := model.DeviceGroupMember{}
	hasMembership, err := db.Where("device_id = ?", device.Id).Get(&membership)
	if err != nil {
		return Effective{}, err
	}
	if hasMembership {
		group := model.DeviceGroup{}
		if has, loadErr := db.ID(membership.GroupId).Where("enabled = ?", true).Get(&group); loadErr != nil {
			return Effective{}, loadErr
		} else if has {
			candidates = append(candidates, struct {
				scope  string
				id     int
				source string
			}{model.StrategyScopeGroup, group.Id, "group:" + group.Name})
		}
	}
	candidates = append(candidates, struct {
		scope  string
		id     int
		source string
	}{model.StrategyScopeGlobal, 0, "global"})
	for _, candidate := range candidates {
		assignment := model.ServerProfileAssignment{}
		has, loadErr := db.Where("scope_type = ? AND scope_id = ?", candidate.scope, candidate.id).Get(&assignment)
		if loadErr != nil {
			return Effective{}, loadErr
		}
		if !has {
			continue
		}
		profile := model.ServerProfile{}
		has, loadErr = db.ID(assignment.ProfileId).Where("enabled = ?", true).Get(&profile)
		if loadErr != nil {
			return Effective{}, loadErr
		}
		if !has {
			continue
		}
		effective.ProfileEnabled = true
		effective.ProfileSource = candidate.source
		effective.Profile = ServerProfile{
			ID: profile.Id, Name: profile.Name, IDServer: profile.IdServer,
			RelayServer: profile.RelayServer, ServerKey: profile.ServerKey,
			PasswordCiphertext: profile.PasswordCiphertext,
		}
		return effective, nil
	}
	return effective, nil
}

func AssignmentProfileID(db *xorm.Engine, scope string, id int) (int, error) {
	assignment := model.ServerProfileAssignment{}
	has, err := db.Where("scope_type = ? AND scope_id = ?", scope, id).Get(&assignment)
	if err != nil || !has {
		return 0, err
	}
	return assignment.ProfileId, nil
}

func ValidateScope(scope string, id int) error {
	if scope == model.StrategyScopeGlobal && id == 0 {
		return nil
	}
	if id > 0 && (scope == model.StrategyScopeGroup || scope == model.StrategyScopeDevice) {
		return nil
	}
	return errors.New("invalid strategy scope")
}
