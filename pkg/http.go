package pirelay

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func (c *Config) WebRoot(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		c.mu.Lock()
		defer c.mu.Unlock()
		json.NewEncoder(w).Encode(c.Relays)
		httpLog(false, fmt.Sprintf("sent %d relays", len(c.Relays)), r)
	case http.MethodPost:
		nrl := &Relay{}
		if err := json.NewDecoder(r.Body).Decode(nrl); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			httpLog(true, err.Error(), r)
			return
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		alreadyHave := ""
		for _, r := range c.Relays {
			switch {
			case r.GPIO == nrl.GPIO:
				alreadyHave = fmt.Sprintf("relay on GPIO %d is already configured", r.GPIO)
			case r.Name == nrl.Name:
				alreadyHave = fmt.Sprintf("relay with name %s is already configured on GPIO %d", nrl.Name, r.GPIO)
			}
		}
		if alreadyHave != "" {
			httpLog(true, alreadyHave, r)
			nrl.httpError(http.StatusConflict, alreadyHave, w, r)
			return
		}
		var err error
		nrl, err = c.NewRelay(nrl.GPIO, nrl.Name, nrl.State, nrl.Schedules, nrl.Sun)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			httpLog(true, err.Error(), r)
			return
		}
		c.Relays = append(c.Relays, nrl)
		http.HandleFunc(fmt.Sprintf("/%d", nrl.GPIO), nrl.WebRelay)
		http.HandleFunc(fmt.Sprintf("/%s", nrl.Name), nrl.WebRelay)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(nrl)
		httpLog(false, fmt.Sprintf("relay %s on GPIO %d created", nrl.Name, nrl.GPIO), r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		httpLog(true, "method not allowed", r)
	}
}

func (rl *Relay) WebRelay(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rl.mu.Lock()
		defer rl.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	case http.MethodPatch:
		rl.Toggle()
		w.WriteHeader(http.StatusOK)
	case http.MethodPost:
		var nrl Relay
		if err := json.NewDecoder(r.Body).Decode(&nrl); err != nil {
			rl.httpError(http.StatusBadRequest, err.Error(), w, r)
			return
		}
		if rl.GPIO != nrl.GPIO {
			rl.httpError(http.StatusExpectationFailed, fmt.Sprintf("GPIO %d does not match current", nrl.GPIO), w, r)
			return
		}
		if nrl.State != rl.State {
			rl.Toggle()
		}
		rl.mu.Lock()
		if nrl.Name != rl.Name {
			rl.Name = nrl.Name
		}
		rl.mu.Unlock()
		if nrl.Sun.Enabled != rl.Sun.Enabled {
			rl.StopSchedules()
			rl.Sun.Enabled = nrl.Sun.Enabled
			if rl.createSchedules(rl.location, rl.timezone) {
				go rl.workSchedules(rl.location, rl.timezone)
			}
		}
	default:
		rl.httpError(http.StatusMethodNotAllowed, "method not allowed", w, r)
		return
	}
	httpLog(false, "OK", r)
	json.NewEncoder(w).Encode(rl)
}

func (rl *Relay) httpError(status int, err string, w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": err,
		"gpio":  rl.GPIO,
	})
}

func httpLog(isErr bool, msg string, r *http.Request) {
	Log(isErr, "%s %s %s: %s", r.RemoteAddr, r.Method, r.URL, msg)
}
