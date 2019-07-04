package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

// Wallet struct
type Wallet struct {
	ID      string `json:"id"`
	Balance int    `json:"balance"`
}

// Transaction struct
type Transaction struct {
	ID    string    `json:"id"`
	From  string    `json:"from"`
	To    string    `json:"to"`
	Value int       `json:"value"`
	Date  time.Time `json:"date"`
}

// Info struct
type Info struct {
	PendingTransactions int64 `json:"pendingTransactions"`
}

func main() {
	r := mux.NewRouter()

	r.HandleFunc("/wallets/{id}", createWallet).Methods("POST")
	r.HandleFunc("/wallets/{id}", getBalance).Methods("GET")

	r.HandleFunc("/transactions", createTransaction).Methods("POST")
	r.HandleFunc("/transactions", getStatement).Methods("GET")

	r.HandleFunc("/info", getInfo).Methods("GET")

	port := getEnvOrDefault("PORT", "8000")

	srv := &http.Server{
		Handler:      r,
		Addr:         fmt.Sprintf("0.0.0.0:%s", port),
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Printf("Listening at %s", port)
	log.Fatal(srv.ListenAndServe())
}

func createWallet(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id := params["id"]

	//TODO: create wallet

	log.Printf("Wallet created: %s", id)
	w.WriteHeader(http.StatusCreated)
}

func getBalance(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id := params["id"]

	//TODO: get balance

	writeJSON(&Wallet{
		ID:      id,
		Balance: 0,
	}, http.StatusOK, w)
}

func createTransaction(w http.ResponseWriter, r *http.Request) {
	var t Transaction
	err := json.NewDecoder(r.Body).Decode(&t)
	if err != nil {
		badRequest("Invalid JSON", w)
		return
	}

	if len(t.To) < 1 || t.Value <= 0 {
		badRequest("Mandatory info missing", w)
		return
	}

	//TODO: create transaction

	json := writeJSON(t, http.StatusCreated, w)
	log.Printf("Transaction created: %s", json)
}

func getStatement(w http.ResponseWriter, r *http.Request) {
	walletID, ok := r.URL.Query()["walletId"]

	if !ok || len(walletID[0]) < 1 {
		badRequest("Missing walledId", w)
		return
	}

	//TODO: get transactions
}

func getInfo(w http.ResponseWriter, r *http.Request) {
	info, err := callEthereumAPI("txpool_status", []interface{}{})
	if err != nil {
		log.Printf("Error consuming Ethereum API: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	pending := info.(map[string]interface{})["pending"]

	writeJSON(&Info{
		PendingTransactions: hexToInt64(pending),
	}, http.StatusOK, w)
}

func callEthereumAPI(method string, params []interface{}) (interface{}, error) {
	addr := getEnvOrDefault("RPC_ADDR", "http://10.209.9.158:8545")

	jsonValue, _ := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "method": method, "params": params, "id": 1})
	request, _ := http.NewRequest("POST", addr, bytes.NewBuffer(jsonValue))
	request.Header.Set("content-type", "application/json")

	client := &http.Client{}
	defer client.CloseIdleConnections()
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	err = json.NewDecoder(response.Body).Decode(&result)
	if err != nil {
		return nil, err
	}

	return result["result"], nil
}

func hexToInt64(hex interface{}) (int64) {
	strHex := fmt.Sprintf("%v", hex)
	strHex = strHex[2:len(strHex)]
	value, _ := strconv.ParseInt(strHex, 16, 32)
	return value
}

func badRequest(reason string, w http.ResponseWriter) {
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(reason))
}

func writeJSON(v interface{}, statusCode int, w http.ResponseWriter) []byte {
	json, _ := json.Marshal(v)
	w.Header().Add("content-type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(json)
	return json
}

func getEnvOrDefault(key string, defaultValue string) string {
	value, set := os.LookupEnv(key)
	if !set {
		value = defaultValue
	}
	return value
}
