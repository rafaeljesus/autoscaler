// Copyright 2018 Drone.IO Inc
// Use of this software is governed by the Business Source License
// that can be found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"

	"github.com/drone/autoscaler"
	"github.com/drone/autoscaler/config"
	"github.com/drone/autoscaler/drivers/digitalocean"
	"github.com/drone/autoscaler/metrics"
	"github.com/drone/autoscaler/scaler"
	"github.com/drone/autoscaler/server"
	"github.com/drone/autoscaler/slack"
	"github.com/drone/autoscaler/store"
	"github.com/drone/drone-go/drone"
	"github.com/drone/signal"

	"github.com/go-chi/chi"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/oauth2"
	"golang.org/x/sync/errgroup"

	_ "github.com/joho/godotenv/autoload"
)

var (
	source  = "https://github.com/drone/autoscaler.git"
	version string
	commit  string
)

func main() {
	conf := config.MustLoad()
	metrics.MinPool(conf)
	metrics.MaxPool(conf)
	setupLogging(conf)

	provider, err := setupProvider(conf)
	if err != nil {
		log.Fatal().Err(err).
			Msg("Invalid or missing hosting provider")
	}

	// instruments the provider with prometheus metrics.
	provider = metrics.ServerCreate(provider)
	provider = metrics.ServerDelete(provider)

	// instruments the provider with slack notifications
	// instance creation and termination events.
	if conf.Slack.Webhook != "" {
		provider = slack.New(conf, provider)
	}

	db := store.Must(conf.Database.Path)
	servers := store.NewServerStore(db)

	// instruments the store with prometheus metrics.
	servers = metrics.ServerCount(servers)
	defer db.Close()

	client := setupClient(conf)

	r := chi.NewRouter()
	r.Use(hlog.NewHandler(log.Logger))
	r.Use(hlog.RemoteAddrHandler("ip"))
	r.Use(hlog.URLHandler("path"))
	r.Use(hlog.MethodHandler("method"))
	r.Use(hlog.RequestIDHandler("request_id", "Request-Id"))

	r.Get("/metrics", server.HandleMetrics(conf.Prometheus.Token))
	r.Get("/version", server.HandleVersion(source, version, commit))
	r.Get("/healthz", server.HandleHealthz())
	r.Route("/api", func(r chi.Router) {
		r.Use(server.CheckDrone(conf))

		r.Get("/queue", server.HandleQueueList(client))
		r.Get("/servers", server.HandleServerList(servers))
		r.Post("/servers", server.HandleServerCreate(servers, provider, conf))
		r.Get("/servers/{name}", server.HandleServerFind(servers))
		r.Delete("/servers/{name}", server.HandleServerDelete(servers, provider))
	})

	//
	// starts the web server.
	//

	srv := &http.Server{
		Handler: r,
	}

	ctx := log.Logger.WithContext(context.Background())
	ctx = signal.WithContextFunc(ctx, func() {
		srv.Shutdown(ctx)
	})

	var g errgroup.Group
	g.Go(func() error {
		if conf.TLS.Autocert {
			return srv.Serve(
				autocert.NewListener(conf.HTTP.Host),
			)
		} else if conf.TLS.Cert != "" {
			return srv.ListenAndServeTLS(
				conf.TLS.Cert,
				conf.TLS.Key,
			)
		}
		srv.Addr = conf.HTTP.Port
		return srv.ListenAndServe()
	})

	//
	// starts the auto-scaler routine.
	//

	g.Go(func() error {
		return scaler.Start(ctx, &scaler.Scaler{
			Client:   client,
			Config:   conf,
			Servers:  servers,
			Provider: provider,
		}, conf.Interval)
	})

	if err := g.Wait(); err != nil {
		log.Fatal().Err(err).Msg("Program terminated")
	}
}

// helper funciton configures the http server.
func setupServer(c config.Config) *http.Server {
	return &http.Server{
		Addr: c.HTTP.Port,
	}
}

// helper funciton configures the logging.
func setupLogging(c config.Config) {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if c.Logs.Debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
	if c.Logs.Pretty {
		log.Logger = log.Output(
			zerolog.ConsoleWriter{
				Out:     os.Stderr,
				NoColor: !c.Logs.Color,
			},
		)
	}
}

// helper function configures the drone client.
func setupClient(c config.Config) drone.Client {
	config := new(oauth2.Config)
	auther := config.Client(
		oauth2.NoContext,
		&oauth2.Token{
			AccessToken: c.Server.Token,
		},
	)
	uri := new(url.URL)
	uri.Scheme = c.Server.Proto
	uri.Host = c.Server.Host
	return drone.NewClient(uri.String(), auther)
}

// helper function configures the hosting provider.
func setupProvider(c config.Config) (autoscaler.Provider, error) {
	switch {
	case c.DigitalOcean.Token != "":
		return digitalocean.FromConfig(c)
	default:
		return nil, errors.New("missing provider configuration")
	}
}
