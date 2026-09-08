package policy

import (
	"rustdesk-api-server-pro/app/model"
	"time"

	"xorm.io/xorm"
)

func CurrentState(db *xorm.Engine) (model.StrategyState, error) {
	state := model.StrategyState{Id: 1}
	has, err := db.ID(state.Id).Get(&state)
	if err != nil {
		return state, err
	}
	if has {
		return state, nil
	}
	state.Revision = time.Now().UnixMilli()
	if _, err = db.Insert(&state); err != nil {
		if has, loadErr := db.ID(state.Id).Get(&state); loadErr == nil && has {
			return state, nil
		}
		return state, err
	}
	return state, nil
}

func CurrentRevision(db *xorm.Engine) (int64, error) {
	state, err := CurrentState(db)
	return state.Revision, err
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
	state, err := CurrentState(db)
	if err != nil {
		return Effective{}, err
	}
	effective := Effective{Revision: state.Revision, RootCommand: "auto", GroupSource: "none"}
	if device.StrategyGroupId > 0 {
		resolved, found, warning, resolveErr := resolveGroup(db, effective, device.StrategyGroupId, "assigned")
		if resolveErr != nil {
			return Effective{}, resolveErr
		}
		if found {
			return resolved, nil
		}
		effective.GroupWarning = warning
		return effective, nil
	}
	if state.DefaultGroupId > 0 {
		resolved, found, warning, resolveErr := resolveGroup(db, effective, state.DefaultGroupId, "default")
		if resolveErr != nil {
			return Effective{}, resolveErr
		}
		if found {
			return resolved, nil
		}
		if effective.GroupWarning == "" {
			effective.GroupWarning = warning
		}
	}
	return effective, nil
}

func resolveGroup(db *xorm.Engine, effective Effective, id int, source string) (Effective, bool, string, error) {
	group := model.DeviceGroup{}
	has, err := db.ID(id).Get(&group)
	if err != nil {
		return Effective{}, false, "", err
	}
	if !has {
		return effective, false, source + "_group_missing", nil
	}
	if !group.Enabled {
		return effective, false, source + "_group_disabled", nil
	}
	profile, has, err := loadEnabledProfile(db, group.ProfileId)
	if err != nil {
		return Effective{}, false, "", err
	}
	if !has {
		return effective, false, source + "_profile_unavailable", nil
	}
	rootCommand := group.RootCommand
	if rootCommand == "" {
		rootCommand = "auto"
	}
	effective.GroupID = group.Id
	effective.GroupName = group.Name
	effective.GroupSource = source
	effective.UnattendedEnabled = group.UnattendedEnabled
	effective.RootCommand = rootCommand
	effective.PasswordCiphertext = group.PasswordCiphertext
	effective.ProfileEnabled = true
	effective.ProfileSource = source + ":" + group.Name
	effective.Profile = ServerProfile{
		ID: profile.Id, Name: profile.Name, IDServer: profile.IdServer,
		RelayServer: profile.RelayServer, ServerKey: profile.ServerKey,
	}
	return effective, true, "", nil
}

func loadEnabledProfile(db *xorm.Engine, id int) (model.ServerProfile, bool, error) {
	profile := model.ServerProfile{}
	if id <= 0 {
		return profile, false, nil
	}
	has, err := db.ID(id).Where("enabled = ?", true).Get(&profile)
	return profile, has, err
}
