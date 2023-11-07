### What is pirelay?
raspberrypi (and others) relay manager with embedded http service and support for scheduling toggles (custom and/or based on sunrise/sunset)

> #### Important Notice
> This project, although working for a while already for my own use cases, is in its early stages and might present unexpected behavior.
> Pull requests for fixes/improvements are, of course, very welcome and appreciated.

#
### Usage:
- build the program or get a [release bin](https://github.com/szampardi/pirelay/releases)
- connect one or more relays to a GPIO pin (or pins)
- create a json configuration file such as:
```json
{
  "listen_addr": ":8011",
  "location": [41.9028,12.4964],
  "Timezone": "Europe/Rome",
  "relays": [
    {
      "name": "lights",
      "gpio": 4,
      "state": 1,
      "sun": {
        "enabled": true,
        "rise_state": 0,
        "rise_offset": "-30m",
        "set_state": 1,
        "set_offset": "30m"
      }
    }
  ]
}
```
- run pirelay as you prefer, for example: `./pirelay -c ./pirelay.json` - an example systemd service file can be found in the doc/ directory of this repository
- watch the relay get toggled on/off at sunset/sunrise or when you configured it to do so

Notes:
- The http service supports a few methods on two paths: root `/` and `/GPIO` or `/relayname`. The methods are:
  * `/` GET: retrieve the currently configured relays and their state,
  * `/` POST: submit a *new* relay configuration,
  * `/{GPIO or name}` GET: retrieve a single relay's configuration, state and schedules,
  * `/{GPIO or name}` POST: submit updated configuration (e.g. a new schedule) for a given relay,
  * `/{GPIO or name}` PATCH: toggle the relay state on/off,
  * (to be implemented) `/{GPIO or name}` DELETE: delete a relay configuration,
- when enabling the sunrise/sunset schedules you _must_ provide a proper pair of coordinates,
- timezone is always required in the configuration file,
- the program does not support https or authentication, use a reverse proxy for those purposes - and make sure you do if you plan on exposing the service to the public internet!

#
### Author
SILVANO ZAMPARDI - 2023

#
### Acknowledgements
Thank you to the developers and contributors of the two modules imported by `pirelay`:
- github.com/stianeikeland/go-rpio
- github.com/nathan-osman/go-sunrise

#
### License(s)
- github.com/szampardi/pirelay [MIT](/docs/LICENSE.md)
- github.com/stianeikeland/go-rpio [MIT](https://github.com/stianeikeland/go-rpio/blob/master/LICENSE)
- github.com/nathan-osman/go-sunrise [MIT](https://github.com/nathan-osman/go-sunrise/blob/master/LICENSE.txt)
