package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/rviscarra/x-remote-viewer/internal/api"
	"github.com/rviscarra/x-remote-viewer/internal/encoding"
	"github.com/rviscarra/x-remote-viewer/internal/rdisplay"
	"github.com/rviscarra/x-remote-viewer/internal/rtc"
)

const (
	httpDefaultPort   = "9000"
	defaultStunServer = "stun:stun.l.google.com:19302"
)

func main() {

	httpPort := flag.String("http.port", httpDefaultPort, "HTTP listen port")
	stunServer := flag.String("stun.server", defaultStunServer, "STUN server URL (stun:)")
	flag.Parse()

	var video rdisplay.Service
	video, err := rdisplay.NewVideoProvider()
	if err != nil {
		log.Fatalf("Can't init video: %v", err)
	}
	_, err = video.Screens()
	if err != nil {
		log.Fatalf("Can't get screens: %v", err)
	}

	var enc encoding.Service
	enc, err = encoding.NewEncoderServiceFor(encoding.H264Codec)
	if err != nil {
		log.Fatalf("Can't create an encoder of codec = %v", encoding.H264Codec)
	}

	var webrtc rtc.Service
	webrtc = rtc.NewPionRtcService(*stunServer, video, enc)

	mux := http.NewServeMux()

	// Endpoint to create a new speech to text session
	mux.Handle("/api/", http.StripPrefix("/api", api.MakeHandler(webrtc, video)))

	// Serve static assets
	mux.Handle("/static/", http.StripPrefix("/static", http.FileServer(http.Dir("./web"))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		http.ServeFile(w, r, "./web/index.html")
	})

	errors := make(chan error, 2)
	go func() {
		log.Printf("Starting signaling server on port %s", *httpPort)
		errors <- http.ListenAndServe(fmt.Sprintf(":%s", *httpPort), mux)
	}()

	go func() {
		interrupt := make(chan os.Signal)
		signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
		errors <- fmt.Errorf("Received %v signal", <-interrupt)
	}()

	err = <-errors
	log.Printf("%s, exiting.", err)
}
