package gollector

import (
	"fmt"
	"github.com/julienschmidt/httprouter"
	"net/http"
	"sync"
)

type HttpApi struct {
	HttpServer *http.Server
	wg         sync.WaitGroup
}

func New(ListenAddress string) HttpApi {
	router := httprouter.New()

	router.GET("/", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		fmt.Fprint(w, "Welcome to the octagon son!\n")
	})

	return HttpApi{
		HttpServer: &http.Server{
			Addr:    ListenAddress,
			Handler: router,
		},
		wg: sync.WaitGroup{},
	}
}

func (h HttpApi) Start() error {
	fmt.Println("Starting http server")

	return h.HttpServer.ListenAndServe()
}
