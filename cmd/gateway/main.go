package main

import (
	"flag"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	c "github.com/RedHatInsights/platform-receptor-controller/internal/controller"
	"github.com/RedHatInsights/platform-receptor-controller/internal/controller/api"
	"github.com/RedHatInsights/platform-receptor-controller/internal/controller/ws"
	"github.com/RedHatInsights/platform-receptor-controller/internal/platform/queue"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

const (
	OPENAPI_SPEC_FILE = "/opt/app-root/src/api/api.spec.file"
)

func main() {
	var wsAddr = flag.String("wsAddr", ":8080", "Hostname:port of the websocket server")
	var mgmtAddr = flag.String("mgmtAddr", ":9090", "Hostname:port of the management server")
	flag.Parse()

	wsConfig := ws.GetWebSocketConfig()
	log.Println("WebSocket configuration:")
	log.Println(wsConfig)

	wsMux := mux.NewRouter()

	cm := c.NewConnectionManager()
	kw := queue.StartProducer(queue.GetProducer())
	kc := queue.GetConsumer()
	rd := c.NewResponseReactorFactory()
	rs := c.NewReceptorServiceFactory(kw)
	md := c.NewMessageDispatcherFactory(kc)
	rc := ws.NewReceptorController(wsConfig, cm, wsMux, rd, md, rs)
	rc.Routes()

	apiMux := mux.NewRouter()

	apiSpecServer := api.NewApiSpecServer(apiMux, OPENAPI_SPEC_FILE)
	apiSpecServer.Routes()

	mgmtServer := api.NewManagementServer(cm, apiMux)
	mgmtServer.Routes()

	jr := api.NewJobReceiver(cm, apiMux, kw)
	jr.Routes()

	go func() {
		log.Println("Starting management web server on", *mgmtAddr)
		if err := http.ListenAndServe(*mgmtAddr, handlers.LoggingHandler(os.Stdout, apiMux)); err != nil {
			log.Fatal("ListenAndServe:", err)
		}
	}()

	go func() {
		log.Println("Starting websocket server on", *wsAddr)
		if err := http.ListenAndServe(*wsAddr, handlers.LoggingHandler(os.Stdout, wsMux)); err != nil {
			log.Fatal("ListenAndServe:", err)
		}
	}()

	signalChan := make(chan os.Signal, 1)

	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Blocking waiting for signal")
	<-signalChan
}
