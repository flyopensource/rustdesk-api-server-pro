package service

import (
	"rustdesk-api-server-pro/app/model"
	"xorm.io/xorm"
)

// Personal entries survive unpublishing; only entries created by management are removed.
func ReleaseManagedPeer(s *xorm.Session, peer *model.Peer) error {
	if peer.ManagedCreated {
		_, err := s.ID(peer.Id).Delete(new(model.Peer))
		return err
	}
	_, err := s.ID(peer.Id).Cols("managed_device_id", "managed_created").Update(&model.Peer{})
	return err
}
