package main

import (
 "encoding/json"
 "log"
 "net/http"
 "os"
 "time"
)

func main() {
 mux := http.NewServeMux()
 mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("Content-Type", "application/json")
  _ = json.NewEncoder(w).Encode(map[string]any{"service":"newmmo-dev","ok":true,"time":time.Now().UTC().Format(time.RFC3339)})
 })
 port := os.Getenv("PORT")
 if port == "" { port = "18086" }
 log.Printf("newmmo development API listening on 127.0.0.1:%s", port)
 log.Fatal(http.ListenAndServe("127.0.0.1:"+port, mux))
}
