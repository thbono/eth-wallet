package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"time"

	"github.com/go-pg/pg"
	"github.com/gorilla/mux"
	"github.com/onrik/ethrpc"
)

// Wallet struct
type Wallet struct {
	ID   string
	Hash string
}

// Balance struct
type Balance struct {
	Balance big.Int `json:"balance"`
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
	PendingTransactions int `json:"pendingTransactions"`
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

	pwd := getEnvOrDefault("WALLETS_PASSWORD", "Sicoob2009")
	result, err := callEthereumAPI("personal_newAccount", []interface{}{pwd})
	if err != nil {
		internalServerError(fmt.Sprintf("Error consuming Ethereum API: %v", err), w)
		return
	}

	db := connectDB()
	defer db.Close()

	stmt, err := db.Prepare(`select count(*) from wallets where id = $1`)
	if err != nil {
		internalServerError(fmt.Sprintf("Error prepating wallet database statement: %v", err), w)
		return
	}

	var count int
	_, err = stmt.QueryOne(pg.Scan(&count), id)
	if err != nil {
		internalServerError(fmt.Sprintf("Error finding wallet in database: %v", err), w)
		return
	}

	if count > 0 {
		badRequest("Duplicated wallet", w)
		return
	}

	err = db.Insert(&Wallet{
		ID:   id,
		Hash: fmt.Sprintf("%v", result),
	})
	if err != nil {
		internalServerError(fmt.Sprintf("Error inserting wallet in database: %v", err), w)
		return
	}

	log.Printf("Wallet created: %s", id)
	w.WriteHeader(http.StatusCreated)
}

func getBalance(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id := params["id"]

	db := connectDB()
	defer db.Close()

	var wallet = &Wallet{ID: id}
	err := db.Select(wallet)
	if err != nil {
		badRequest(fmt.Sprintf("Cannot find wallet in database: %v", err), w)
	}

	addr := getEnvOrDefault("RPC_ADDR", "http://10.209.9.158:8545")
	client := ethrpc.New(addr)

	balance, err := client.EthGetBalance(wallet.Hash, "latest")

	writeJSON(&Balance{
		Balance: balance,
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

	t.Date = time.Now()
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

	t := []Transaction{}
	writeJSON(t, http.StatusOK, w)
}

func getInfo(w http.ResponseWriter, r *http.Request) {
	info, err := callEthereumAPI("txpool_status", []interface{}{})
	if err != nil {
		internalServerError(fmt.Sprintf("Error consuming Ethereum API: %v", err), w)
		return
	}

	pending := info.(map[string]interface{})["pending"]

	writeJSON(&Info{
		PendingTransactions: hexToInt64(pending),
	}, http.StatusOK, w)
}

func connectDB() *pg.DB {
	return pg.Connect(&pg.Options{
		Addr:     getEnvOrDefault("DB_ADDR", "10.209.9.158:5432"),
		User:     getEnvOrDefault("DB_USER", "eth"),
		Password: getEnvOrDefault("DB_PASSWORD", "eth"),
		Database: getEnvOrDefault("DB_DATABASE", "eth"),
	})
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

func hexToInt64(hex interface{}) int {
	strHex := fmt.Sprintf("%v", hex)
	value, _ := ethrpc.ParseInt(strHex)
	return value
}

func badRequest(reason string, w http.ResponseWriter) {
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(reason))
}

func internalServerError(reason string, w http.ResponseWriter) {
	log.Println(reason)
	w.WriteHeader(http.StatusInternalServerError)
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
