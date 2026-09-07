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
	rootCommand := device.RootCommand
	if rootCommand == "" {
		rootCommand = "auto"
	}
	effective := Effective{
		Revision:          state.Revision,
		UnattendedEnabled: device.UnattendedEnabled,
		RootCommand:       rootCommand,
	}
	if profile, has, loadErr := loadEnabledProfile(db, device.StrategyProfileId); loadErr != nil {
		return Effective{}, loadErr
	} else if has {
		return withProfile(effective, profile, "device"), nil
	}
	if device.StrategyGroupId > 0 {
		group := model.DeviceGroup{}
		if has, loadErr := db.ID(device.StrategyGroupId).Where("enabled = ?", true).Get(&group); loadErr != nil {
			return Effective{}, loadErr
		} else if has {
			if profile, profileFound, profileErr := loadEnabledProfile(db, group.ProfileId); profileErr != nil {
				return Effective{}, profileErr
			} else if profileFound {
				return withProfile(effective, profile, "group:"+group.Name), nil
			}
		}
	}
	if profile, has, loadErr := loadEnabledProfile(db, state.GlobalProfileId); loadErr != nil {
		return Effective{}, loadErr
	} else if has {
		return withProfile(effective, profile, "global"), nil
	}
	return effective, nil
}

func loadEnabledProfile(db *xorm.Engine, id int) (model.ServerProfile, bool, error) {
	profile := model.ServerProfile{}
	if id <= 0 {
		return profile, false, nil
	}
	has, err := db.ID(id).Where("enabled = ?", true).Get(&profile)
	return profile, has, err
}

func withProfile(effective Effective, profile model.ServerProfile, source string) Effective {
	effective.ProfileEnabled = true
	effective.ProfileSource = source
	effective.Profile = ServerProfile{
		ID: profile.Id, Name: profile.Name, IDServer: profile.IdServer,
		RelayServer: profile.RelayServer, ServerKey: profile.ServerKey,
		PasswordCiphertext: profile.PasswordCiphertext,
	}
	return effective
}
