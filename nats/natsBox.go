package nats

import (
	"sync"

	"github.com/nats-io/nats.go"
)

var natsManager *NatsManager



type NatsManager struct {
	nats  map[string]*Nats
	mutex sync.RWMutex
}

func (d *NatsManager) Read(key string) (*Nats, bool) {
	d.mutex.RLock()

	defer d.mutex.RUnlock()
	val, exists := d.nats[key]
	return val, exists
}

func (d *NatsManager) write(key string, value *Nats) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.nats[key] = value
}

func (d *NatsManager) Delete(key string) {
	d.mutex.RLock()

	defer d.mutex.RUnlock()
	delete(d.nats, key)
}

func (d *NatsManager) GetConnection(key string) *nats.Conn {
	val, exists := d.Read(key)

	if exists {

		if val.con == nil || val.con.IsClosed() {
			d.Delete(key)
			return nil
		}
	}
	if val != nil {
		return val.con
	}
	return nil
}
func GetNatsBox() *NatsManager {
	if natsManager == nil {
		natsManager = &NatsManager{nats: make(map[string]*Nats)}
	}
	return natsManager
}
