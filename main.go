package main

import (
	"GoMldy/handlers"
	"GoMldy/utils"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"log"
	"net/http"
)

func main() {
	utils.LoadEnv()

	utils.InitDb()
	r := chi.NewRouter()

	r.Use(cors.Handler(cors.Options{
		AllowOriginFunc: func(r *http.Request, origin string) bool {
			return utils.RegexCORS(origin)
		},
		AllowCredentials: true,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Origin", "Accept", "Content-Disposition"},
		ExposedHeaders:   []string{"Content-Disposition"},
	}))
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Post("/api/download", handlers.Download)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte("Hello World"))
		if err != nil {
			return
		}
	})

	r.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./favicon.ico")
	})

	log.Println("Server starting on port 9000...")
	srv := &http.Server{
		Addr:    ":9000",
		Handler: h2c.NewHandler(r, &http2.Server{}),
	}
	err := srv.ListenAndServe()
	if err != nil {
		panic("Failed to start server: " + err.Error())
	}
}
