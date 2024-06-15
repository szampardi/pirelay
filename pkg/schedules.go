package pirelay

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/nathan-osman/go-sunrise"
)

type (
	RelaySchedule struct {
		Name          string        `json:"name"`
		At            time.Duration `json:"at"` // after midnight local time
		atTicker      *time.Ticker
		State         RelayState    `json:"state"`
		StateDuration time.Duration `json:"for"`
	}
	Schedules map[string]*RelaySchedule
	Sun       struct {
		Enabled    bool          `json:"enabled"`
		RiseState  RelayState    `json:"rise_state"`
		RiseOffset time.Duration `json:"rise_offset"`
		SetState   RelayState    `json:"set_state"`
		SetOffset  time.Duration `json:"set_offset"`
	}
)

func (r *Relay) ListSchedules(tz string) map[string]time.Time {
	out := make(map[string]time.Time)
	r.mu.Lock()
	defer r.mu.Unlock()
	_, midnight := nowAndMidnight(tz)
	for sn, sc := range r.Schedules {
		out[sn] = midnight.Add(sc.At)
	}
	return out
}

func (r *Relay) AddSchedule(sc RelaySchedule, tz string) {
	now, midnight := nowAndMidnight(tz)
	if midnight.Add(sc.At).After(now) {
		midnight = midnight.Add(time.Hour * 24)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Schedules[sc.Name] = &RelaySchedule{
		Name:          sc.Name,
		At:            sc.At,
		atTicker:      time.NewTicker(midnight.Add(sc.At).Sub(now)),
		State:         sc.State,
		StateDuration: sc.StateDuration,
	}
	Log(false, "added schedule for relay %s: %v", r.Name, sc)
}

func (r *Relay) RemoveSchedule(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for sn, sc := range r.Schedules {
		if name == sn {
			sc.atTicker.Stop()
			return nil
		}
	}
	return fmt.Errorf("schedule %s for relay %s not found", name, r.Name)
}

func (r *Relay) StopSchedules() {
	r.stopSchedules <- true
}

func (r *Relay) StartSchedules(location [2]float64, timezone string) {
	if r.createSchedules(location, timezone) {
		go r.workSchedules(location, timezone)
	}
}

func (r *Relay) workSchedules(location [2]float64, tz string) {
	now, midnight := nowAndMidnight(tz)
	midnightTicker := time.NewTicker(midnight.Add(24 * time.Hour).Sub(now))
loop:
	for {
		for sn, sc := range r.Schedules {
			select {
			case <-sc.atTicker.C:
				Log(false, "running schedule %s for relay %s at %v", sn, r.Name, sc.At.String())
				r.TimedToggle(sc.StateDuration)
				sc.atTicker.Stop()
			default:
			}
		}
		select {
		case <-r.stopSchedules:
			for _, sc := range r.Schedules {
				sc.atTicker.Stop()
			}
			return
		case <-midnightTicker.C:
			for _, sc := range r.Schedules {
				sc.atTicker.Stop()
			}
			break loop
		default:
			time.Sleep(1 * time.Second)
		}
	}
	Log(false, "updating schedules for relay %s", r.Name)
	r.updateSchedules(location, tz)
	goto loop
}

func (r *Relay) createSchedules(location [2]float64, tz string) bool {
	if r.Schedules == nil {
		r.Schedules = make(Schedules)
	}
	now, midnight := nowAndMidnight(tz)
	r.mu.Lock()
	defer r.mu.Unlock()
	for sn, sc := range r.Schedules {
		r.Schedules[sn] = &RelaySchedule{
			Name:  sn,
			At:    sc.At,
			State: sc.State,
		}
		if now.After(midnight.Add(sc.At)) {
			r.Schedules[sn].atTicker = time.NewTicker(midnight.Add(time.Hour * 24).Add(sc.At).Sub(now))
		} else {
			r.Schedules[sn].atTicker = time.NewTicker(midnight.Add(sc.At).Sub(now))
		}
	}
	if r.Sun.Enabled && !(location[0] == 0 && location[1] == 0) {
		rise, set := computeSunTimes(location, r.Sun.RiseOffset, r.Sun.SetOffset)
		r.Schedules["sunrise"] = &RelaySchedule{
			Name:  "sunrise",
			At:    rise.Sub(midnight),
			State: r.Sun.RiseState,
		}
		if now.After(rise) {
			r.Schedules["sunrise"].atTicker = time.NewTicker(rise.Add(time.Hour * 24).Sub(now))
		} else {
			r.Schedules["sunrise"].atTicker = time.NewTicker(rise.Sub(now))
		}
		r.Schedules["sunset"] = &RelaySchedule{
			Name:  "sunset",
			At:    set.Sub(midnight),
			State: r.Sun.SetState,
		}
		if now.After(set) {
			r.Schedules["sunset"].atTicker = time.NewTicker(set.Add(time.Hour * 24).Sub(now))
		} else {
			r.Schedules["sunset"].atTicker = time.NewTicker(set.Sub(now))
		}
		if now.After(rise) && now.Before(set) {
			r.set(r.Sun.RiseState)
			Log(false, "sun is up, turned relay to %s", r.State.String())
		} else {
			r.set(r.Sun.SetState)
			Log(false, "sun is down, turn relay %s", r.State.String())
		}
	} else {
		delete(r.Schedules, "sunrise")
		delete(r.Schedules, "sunset")
	}
	return len(r.Schedules) > 0
}

func (r *Relay) updateSchedules(location [2]float64, tz string) {
	now, midnight := nowAndMidnight(tz)
	var rise, set time.Time
	if r.Sun.Enabled && !(location[0] == 0 && location[1] == 0) {
		rise, set = computeSunTimes(location, r.Sun.RiseOffset, r.Sun.SetOffset)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for sn, sc := range r.Schedules {
		switch sn {
		case "sunrise":
			if !r.Sun.Enabled || rise.IsZero() {
				continue
			}
			r.Schedules[sn] = &RelaySchedule{
				Name:  sn,
				At:    rise.Sub(midnight),
				State: r.Sun.RiseState,
			}
			if now.After(rise) {
				r.Schedules[sn].atTicker = time.NewTicker(rise.Add(time.Hour * 24).Sub(now))
			} else {
				r.Schedules[sn].atTicker = time.NewTicker(rise.Sub(now))
			}
		case "sunset":
			if !r.Sun.Enabled || set.IsZero() {
				continue
			}
			r.Schedules[sn] = &RelaySchedule{
				Name:  sn,
				At:    set.Sub(midnight),
				State: r.Sun.SetState,
			}
			if now.After(set) {
				r.Schedules[sn].atTicker = time.NewTicker(set.Add(time.Hour * 24).Sub(now))
			} else {
				r.Schedules[sn].atTicker = time.NewTicker(set.Sub(now))
			}
		default:
			r.Schedules[sn] = &RelaySchedule{
				Name:  sn,
				At:    sc.At,
				State: sc.State,
			}
			if now.After(midnight.Add(r.Schedules[sn].At)) {
				r.Schedules[sn].atTicker = time.NewTicker(midnight.Add(time.Hour * 24).Add(sc.At).Sub(now))
			} else {
				r.Schedules[sn].atTicker = time.NewTicker(midnight.Add(sc.At).Sub(now))
			}
		}
	}
}

func nowAndMidnight(tz string) (time.Time, time.Time) {
	loc, _ := time.LoadLocation(tz)
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	return now, midnight
}

func computeSunTimes(location [2]float64, riseOffset, setOffset time.Duration) (time.Time, time.Time) {
	t := time.Now()
	rise, set := sunrise.SunriseSunset(location[0], location[1], t.Year(), t.Month(), t.Day())
	return rise.Add(riseOffset), set.Add(setOffset)
}

func (sc *RelaySchedule) UnmarshalJSON(data []byte) error {
	var x map[string]interface{}
	if err := json.Unmarshal(data, &x); err != nil {
		return err
	}
	if v, ok := x["name"]; ok {
		sc.Name = v.(string)
	} else {
		return fmt.Errorf("missing name for schedule")
	}
	if v, ok := x["state"]; ok {
		sc.State = RelayState(v.(float64))
	} else {
		sc.State = StateOFF
	}
	if v, ok := x["at"]; ok {
		t, err := time.ParseDuration(v.(string))
		if err != nil {
			return err
		}
		sc.At = t
	} else {
		return fmt.Errorf("at for schedule cannot be 0")
	}
	return nil
}

func (sc *RelaySchedule) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"name":  sc.Name,
		"at":    sc.At.String(),
		"state": sc.State,
	})
}

func (s *Sun) UnmarshalJSON(data []byte) error {
	var x map[string]interface{}
	var err error
	if err := json.Unmarshal(data, &x); err != nil {
		return err
	}
	if v, ok := x["enabled"]; ok {
		s.Enabled = v.(bool)
	} else {
		return fmt.Errorf("missing enabled setting for sun config")
	}
	if v, ok := x["rise_state"]; ok {
		s.RiseState, err = ParseState(v)
		if err != nil {
			return err
		}
	} else if s.Enabled {
		return fmt.Errorf("missing state setting for sunrise")
	}
	if v, ok := x["rise_offset"]; ok {
		t, err := time.ParseDuration(v.(string))
		if err != nil {
			return err
		}
		s.RiseOffset = t
	} else {
		s.RiseOffset = 0
	}
	if v, ok := x["set_state"]; ok {
		s.SetState, err = ParseState(v)
		if err != nil {
			return err
		}
	} else if s.Enabled {
		return fmt.Errorf("missing state setting for sunset")
	}
	if v, ok := x["set_offset"]; ok {
		t, err := time.ParseDuration(v.(string))
		if err != nil {
			return err
		}
		s.SetOffset = t
	} else {
		s.SetOffset = 0
	}
	return nil
}

func (s *Sun) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"enabled":     s.Enabled,
		"rise_state":  s.RiseState,
		"rise_offset": s.RiseOffset.String(),
		"set_state":   s.SetState,
		"set_offset":  s.SetOffset.String(),
	})
}
