package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

func HttpServer() {
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/manifest", manifestHandler)
	http.HandleFunc("/info/{id}", infoHandler)
	http.HandleFunc("/connect/{id}", connectHandler)
	http.HandleFunc("/disconnect/{id}", disconnectHandler)
	http.HandleFunc("/shell/{id}", shellHandler)
	http.HandleFunc("/keepalive/{id}", keepaliveHandler)

	slog.Info(fmt.Sprint("Start API server on port ", CFG.APIPort))
	err := http.ListenAndServe(fmt.Sprint(":", CFG.APIPort), nil)
	if err != nil {
		panic(err)
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "rlpa-server\n")
}

func manifestHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "manifest\n")
}

func infoHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if verify(id, r.Header.Get("Password")) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, fmt.Sprint("hello ", id))
	} else {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(w, "Unauthorized")
	}
}

func connectHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	passwd := r.Header.Get("Password")
	if verify(id, passwd) {
		c, err := FindClient(id)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "rlpa client disconnected")
			return
		}
		if c.WorkMode != ShellMode {
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprintf(w, "rlpa client not in shell mode")
			return
		}
		if c.APILocked {
			w.WriteHeader(http.StatusConflict)
			return
		}
		c.APILocked = true
		c.StartOrResetTimer()
		w.WriteHeader(http.StatusOK)
		return
	} else {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
}

func disconnectHandler(w http.ResponseWriter, r *http.Request) {
	// TODO 释放连接
}

func keepaliveHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	passwd := r.Header.Get("Password")
	if verify(id, passwd) {
		c, err := FindClient(id)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "rlpa client disconnected")
			return
		}
		if !c.APILocked || c.KeepAliveTimer == nil {
			w.WriteHeader(http.StatusTeapot)
			return
		}
		c.StartOrResetTimer()
		return
	} else {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
}

func shellHandler(w http.ResponseWriter, r *http.Request) {
	// TODO 执行 shell
	id := r.PathValue("id")
	passwd := r.Header.Get("Password")
	if verify(id, passwd) {
		c, err := FindClient(id)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "rlpa client disconnected")
			return
		}
		if c.WorkMode != ShellMode {
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprintf(w, "rlpa client not in shell mode")
			return
		}
		// decode json body
		var payload ShellRequest
		err = json.NewDecoder(r.Body).Decode(&payload)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "bad request")
			return
		}
		switch payload.Type {
		case TypeFinish:
			c.Close(ResultFinished)
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "Closed")
			return
		case TypeExecute:
			if c.ResponseWaiting {
				w.WriteHeader(http.StatusConflict)
				fmt.Fprintf(w, "already has one lpac shell running")
				return
			}
			c.DebugLog("command " + payload.Command)
			err = c.processOpenLpac(strings.Split(strings.TrimSpace(payload.Command), " "))
			if err != nil {
				w.WriteHeader(http.StatusBadGateway)
				fmt.Fprintf(w, "failed to open lpac")
				return
			}
			c.ResponseWaiting = true
			resp := <-c.ResponseChan
			c.ResponseWaiting = false
			fmt.Fprintf(w, string(resp))
			return
		}

	} else {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(w, "Unauthorized")
	}
}

func verify(id, passwd string) bool {
	if v, exists := Credentials[id]; exists {
		if passwd == v {
			return true
		}
		return false
	}
	return false
}
