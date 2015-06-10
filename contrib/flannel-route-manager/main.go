package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/kelseyhightower/flannel-route-manager/backend"
	"github.com/kelseyhightower/flannel-route-manager/backend/google"
	"github.com/kelseyhightower/flannel-route-manager/server"
)

var (
	backendName  string
	etcdEndpoint string
	etcdPrefix   string
	deleteRoutes bool
	syncInterval int
)

func init() {
	flag.StringVar(&backendName, "backend", "google", "backend provider")
	flag.StringVar(&etcdEndpoint, "etcd-endpoint", "http://127.0.0.1:4001", "etcd endpoint")
	flag.StringVar(&etcdPrefix, "etcd-prefix", "/coreos.com/network", "etcd prefix")
	flag.BoolVar(&deleteRoutes, "delete-all-routes", false, "delete all flannel routes")
	flag.IntVar(&syncInterval, "sync-interval", 300, "sync interval")
}

func main() {
	flag.Parse()
	log.SetFlags(log.LstdFlags)
	var routeManager backend.RouteManager
	var err error
	switch backendName {
	case "google":
		routeManager, err = google.New()
		if err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatal("unknown backend ", backendName)
	}
	if deleteRoutes {
		log.Println("deleting all routes")
		routes, err := routeManager.DeleteAllRoutes()
		if routes != nil {
			for _, r := range routes {
				log.Printf("deleted: %s\n", r)
			}
		}
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
		os.Exit(0)
	}
	log.Println("starting fleet route manager...")
	s := server.New(etcdEndpoint, etcdPrefix, syncInterval, routeManager).Start()
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	c := <-signalChan
	log.Println(fmt.Sprintf("captured %v exiting...", c))
	s.Stop()
}
