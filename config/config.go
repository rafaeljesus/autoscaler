// Copyright 2018 Drone.IO Inc
// Use of this software is governed by the Business Source License
// that can be found in the LICENSE file.

package config

import "time"

type (
	// Config stores the configuration settings.
	Config struct {
		Interval time.Duration `default:"5m"`

		Slack struct {
			Webhook string
		}

		Logs struct {
			Color  bool
			Debug  bool
			Pretty bool
		}

		Pool struct {
			Min    int           `default:"2"`
			Max    int           `default:"4"`
			MinAge time.Duration `default:"60m" split_words:"true"`
		}

		Server struct {
			Host  string
			Proto string
			Token string
		}

		Agent struct {
			Host        string
			Token       string
			Image       string
			Concurrency int
		}

		HTTP struct {
			Host string
			Port string `default:":8080"`
		}

		TLS struct {
			Autocert bool
			Cert     string
			Key      string
		}

		Prometheus struct {
			Token string
		}

		Database struct {
			Path string `default:"snapshot.db"`
		}

		DigitalOcean struct {
			Token  string
			Image  string `default:"docker-16-04"`
			Region string `default:"nyc3"`
			SSHKey string
			Size   string `default:"s-1vcpu-1gb"`
			IPv6   bool
			Tags   []string
		}

		Google struct {
			Zone         string `default:"us-central1-a"`
			MachineType  string `split_words:"true" default:"n1-standard-1"`
			MachineImage string `split_words:"true" default:"ubuntu-1510-wily-v20151114"`
			DiskType     string `split_words:"true" default:"pd-standard"`
			Address      string
			Network      string `default:"default"`
			Subnetwork   string
			Preemptible  bool
			Scopes       string
			DiskSize     int `split_words:"true"`
			Project      string
			Tags         []string
		}
	}
)
