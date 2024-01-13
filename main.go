// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"
)

var addr = flag.String("addr", ":8080", "http service address")

func serveHome(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL)
	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.ServeFile(w, r, "home.html")
}

func scroll(w http.ResponseWriter, r *http.Request) {
}

func sleep(w http.ResponseWriter, r *http.Request) {
	time.Sleep(5 * time.Second)
	render(w, "", nil)
}

func typing(w http.ResponseWriter, r *http.Request) {
	render(w, `<div hx-trigger="load" hx-include="#typing" ws-send></div>`, nil)
}

func changescreen(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		render(w, `<form id="screenname" ws-send style="float: right; display: inline-flex;">
	Name: <input name="screenname" type="text" autofocus style="margin-right: 1.1em;">
</form>`, nil)
	}
}

func main() {
	flag.Parse()
	hub := newHub()
	go hub.run()
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/scroll", scroll)
	http.HandleFunc("/typing", typing)
	http.HandleFunc("/sleep", sleep)
	http.HandleFunc("/changescreen", changescreen)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})
	server := &http.Server{
		Addr:              *addr,
		ReadHeaderTimeout: 3 * time.Second,
	}
	err := server.ListenAndServe()
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func render(w http.ResponseWriter, html string, data any) {
	// Render the HTML template
	// fmt.Println("Rendering...")
	w.WriteHeader(http.StatusOK)
	tmpl, err := template.New(html).Parse(html)
	if err != nil {
		fmt.Println(err)
	}
	err = tmpl.Execute(w, data)
	if err != nil {
		fmt.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
