package pirelay

import (
	"fmt"
	"sync"
	"time"

	"github.com/stianeikeland/go-rpio/v4"
)

type (
	RelayChange struct {
		State RelayState    `json:"state"`
		For   time.Duration `json:"for"`
	}
	RelayState int
	Relay      struct {
		Name      string     `json:"name"`
		GPIO      int        `json:"gpio"`
		State     RelayState `json:"state"`
		Schedules Schedules  `json:"schedules"`
		Sun       Sun        `json:"sun"`

		timezone string
		location [2]float64

		changes       chan *RelayChange
		stopSchedules chan bool

		pin rpio.Pin
		mu  *sync.Mutex
	}
)

const (
	StateToggle RelayState = iota - 1
	StateOFF
	StateON
)

func (s RelayState) String() string {
	switch s {
	case StateON:
		return "ON"
	case StateOFF:
		return "OFF"
	}
	return ""
}

func ParseState(s interface{}) (RelayState, error) {
	var st RelayState
	var err error
	switch t := s.(type) {
	case bool:
		if t {
			st = StateON
		} else {
			st = StateOFF
		}
	case string:
		switch t {
		case "on", "ON":
			st = StateON
		case "off", "OFF":
			st = StateOFF
		default:
			err = fmt.Errorf("invalid state string %v", t)
		}
	case float64:
		i := RelayState(t)
		switch i {
		case StateON:
			st = i
		case StateOFF:
			st = i
		default:
			err = fmt.Errorf("invalid state float64 %v", t)
		}
	case RelayState:
		st = t
	default:
		err = fmt.Errorf("invalid state type %v", t)
	}
	return st, err
}

func (r *Relay) set(s RelayState) {
	switch s {
	case StateOFF:
		r.pin.Low()
		r.State = StateOFF
	case StateON:
		r.pin.High()
		r.State = StateON
	}
	Log(false, "turned %s relay %s on GPIO %d", r.State, r.Name, r.GPIO)
}

func (r *Relay) TimedOn(dur time.Duration) {
	r.On()
	if dur != 0 {
		go func() {
			time.Sleep(dur)
			r.Off()
		}()
	}
}

func (r *Relay) On() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.set(StateON)
}

func (r *Relay) TimedOff(dur time.Duration) {
	r.Off()
	if dur != 0 {
		go func() {
			time.Sleep(dur)
			r.On()
		}()
	}
}

func (r *Relay) Off() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.set(StateOFF)
}

func (r *Relay) TimedToggle(dur time.Duration) {
	r.Toggle()
	if dur != 0 {
		go func() {
			time.Sleep(dur)
			r.Toggle()
		}()
	}
}

func (r *Relay) Toggle() {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch r.State {
	case StateOFF:
		r.set(StateON)
	case StateON:
		r.set(StateOFF)
	}
}

func (c *Config) NewRelay(gpio int, name string, opts ...interface{}) (*Relay, error) {
	mode := rpio.Output
	r := &Relay{
		Name:          name,
		GPIO:          gpio,
		Schedules:     make(Schedules),
		stopSchedules: make(chan bool, 1),
		location:      c.Location,
		timezone:      c.Timezone,
		pin:           rpio.Pin(gpio),
		mu:            &sync.Mutex{},
	}
	if r.Name == "" {
		r.Name = fmt.Sprintf("GPIO%d", gpio)
	}
	for _, i := range opts {
		switch t := i.(type) {
		case RelayState:
			r.State = t
		case Sun:
			r.Sun = t
		case Schedules:
			r.Schedules = t
		case RelaySchedule:
			r.Schedules[t.Name] = &t
		case rpio.Mode:
			mode = t
		case [2]float64:
			r.location = t
		case string:
			r.timezone = t
		case chan *RelayChange:
			r.changes = t
			go func() {
				select {
				case change := <-r.changes:
					switch change.State {
					case StateToggle:
						r.TimedToggle(change.For)
					case StateOFF:
						r.TimedOff(change.For)
					case StateON:
						r.TimedOn(change.For)
					}
				case <-r.stopSchedules:
					return
				}
			}()
		}
	}
	if r.Sun.Enabled && (r.location[0] == 0 && r.location[1] == 0) {
		return nil, fmt.Errorf("location coordinates and timezone must be provided to work sunset/sunrise schedules")
	}
	if r.timezone == "" {
		return nil, fmt.Errorf("timezone must be provided to work/update schedules")
	}
	r.pin.Mode(mode)
	r.set(r.State)
	if r.createSchedules(r.location, r.timezone) {
		go r.workSchedules(r.location, r.timezone)
	}
	Log(false, "added relay %s on GPIO %d with initial state %v", r.Name, r.GPIO, r.State)
	return r, nil
}
