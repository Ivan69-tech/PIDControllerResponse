package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regulation/simulation"
)

type DataReceived struct {
	Sp  float64 `json:"Sp"`
	Tau float64 `json:"Tau"`
	K   float64 `json:"K"`
	P   float64 `json:"P"`
	Ki  float64 `json:"Ki"`
	Kd  float64 `json:"Kd"`
	Dt  float64 `json:"dt"`
	N   float64 `json:"N"`
}

func getDataHandler(w http.ResponseWriter, r *http.Request) {

	var data DataReceived
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, "Erreur lors du décodage de la donnée", http.StatusBadRequest)
		fmt.Println(err)
		return
	}

	fmt.Println("Donnée reçue:", data)
	T, res := simulation.Simulation(
		data.Sp,
		data.Tau,
		data.K,
		data.P,
		data.Ki,
		data.Kd,
		data.Dt,
		data.N)

	response := map[string][]float64{
		"X": T,
		"Y": res,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

//go:embed static/html/*.html
//go:embed static/js/*.js

var content embed.FS

func main() {

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	http.HandleFunc("/sendData", getDataHandler)
	fs := http.FileServer(http.Dir("./static/html"))
	http.Handle("/", http.StripPrefix("/", fs))

	log.Println("Serveur démarré sur http://localhost:8080")
	log.Fatal(http.ListenAndServe(":2222", nil))
}
