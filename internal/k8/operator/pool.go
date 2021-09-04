package operator

import (
	log "github.com/sirupsen/logrus"
	"net"
)

type pool struct {}

func NewPool() *pool {
	return &pool{}
}

func (p *pool) Add(idx int, id string, add net.IP) error {
	log.Infof("add idx %d ID %s ip %s", idx, id, add.String())
	return nil
}