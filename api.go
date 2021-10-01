package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"golang.org/x/net/websocket"
)

type ResponseCur struct {
	ID          string `json:"id"`
	FullName    string `json:"fullName"`
	FeeCurrency string `json:"feeCurrency"`
	Ask         string `json:"ask"`
	Bid         string `json:"bid"`
	Last        string `json:"last"`
	Open        string `json:"open"`
	Low         string `json:"low"`
	High        string `json:"high"`
}

type Currency struct {
	mu   sync.Mutex
	Data map[string]ResponseCur
}

var cur *Currency = &Currency{}

func (c *Currency) Add(key string, res ResponseCur) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Data[key] = res
	
}

func (c *Currency) Get(key string) (res ResponseCur, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	res, ok = c.Data[key]
	return
}

func (c *Currency) GetAll() (res map[string]ResponseCur) {
	c.mu.Lock()
	defer c.mu.Unlock()
	res = c.Data
	return
}

type HitbtcRequest struct {
	Method string `json:"method"`
	Params struct {
		Currency string `json:"currency"`
		Symbol   string `json:"symbol"`
	} `json:"params"`
}
type HitbtcGetSymbolResponse struct {
	Result struct {
		BaseCurrency string `json:"baseCurrency"`
		FeeCurrency  string `json:"feeCurrency"`
	} `json:"result"`
}
type HitbtcGetCurrencyResponse struct {
	Result struct {
		ID       string `json:"id"`
		FullName string `json:"fullName"`
	} `json:"result"`
}

type HitbtcResponse struct {
	Params struct {
		Ask  string `json:"ask"`
		Bid  string `json:"bid"`
		Last string `json:"last"`
		Open string `json:"open"`
		Low  string `json:"low"`
		High string `json:"high"`
	} `json:"params"`
}

//constants
const (
	Origin = "https://localhost/"
	Url    = "wss://api.hitbtc.com/api/2/ws"
)

type CurrencySymbols struct {
	Symbolscur []string
}

func GetAllCurrency(w http.ResponseWriter, rq *http.Request) {
	response, _ := json.Marshal(cur.GetAll())
	io.WriteString(w, string(response))

}

func GetCurrencyForID(w http.ResponseWriter, rq *http.Request) {
	id := strings.TrimPrefix(rq.URL.Path, "/currency/")
	value, ok := cur.Get(id)
	if ok {
		response, _ := json.Marshal(value)
		io.WriteString(w, string(response))
	} else {
		io.WriteString(w, "id is not valid")
	}

}

func GetWebsocketsConnection() (ws *websocket.Conn) {
	ws, err := websocket.Dial(Url, "", Origin)
	if err != nil {
		log.Fatal(err)
	}
	return ws
}
func SendRecWSdata(ws *websocket.Conn, req interface{}) (res []byte) {
	b, _ := json.Marshal(&req)
	if _, err := ws.Write(b); err != nil {
		log.Fatal(err)
	}
	var msg = make([]byte, 512)
	var n int
	n, err := ws.Read(msg)
	if err != nil {
		log.Fatal(err)
	}
	return msg[:n]
}
func GetTickerData(symbol string) {
	ws := GetWebsocketsConnection()
	defer ws.Close()
	for {
		hitRequest := HitbtcRequest{}
		hitRequest.Method = "subscribeTicker"
		hitRequest.Params.Symbol = symbol
		var hitbtcResponse HitbtcResponse
		b := SendRecWSdata(ws, hitRequest)
		_ = json.Unmarshal(b, &hitbtcResponse)
		if len(hitbtcResponse.Params.Ask) > 0 {
			if data, ok := cur.Get(symbol); ok {
				data.Ask = hitbtcResponse.Params.Ask
				data.Bid = hitbtcResponse.Params.Bid
				data.High = hitbtcResponse.Params.High
				data.Last = hitbtcResponse.Params.Last
				data.Low = hitbtcResponse.Params.Low
				data.Open = hitbtcResponse.Params.Open
				cur.Add(symbol, data)
			} else {
				fmt.Println("Symbol not valid")
			}
		}
	}
}
func GetHitBtcdata(symbol string) {
	ws := GetWebsocketsConnection()
	defer ws.Close()
	if _, ok := cur.Get(symbol); !ok {
		getSymbols := HitbtcRequest{}
		var result ResponseCur
		getSymbols.Method = "getSymbol"
		getSymbols.Params.Symbol = symbol
		var hitbtcGetSymbolResponse HitbtcGetSymbolResponse
		_ = json.Unmarshal(SendRecWSdata(ws, getSymbols), &hitbtcGetSymbolResponse)
		result.FeeCurrency = hitbtcGetSymbolResponse.Result.FeeCurrency
		getSymbols.Method = "getCurrency"
		getSymbols.Params.Currency = hitbtcGetSymbolResponse.Result.BaseCurrency
		var hitbtcGetCurrencyResponse HitbtcGetCurrencyResponse
		_ = json.Unmarshal(SendRecWSdata(ws, getSymbols), &hitbtcGetCurrencyResponse)
		result.ID = hitbtcGetCurrencyResponse.Result.ID
		result.FullName = hitbtcGetCurrencyResponse.Result.FullName
		cur.Add(symbol, result)
		go GetTickerData(symbol)
	}
}
func main() {
	m := map[string]ResponseCur{}
	cur.Data = m
	file, _ := os.Open("conf.json")
	defer file.Close()
	decoder := json.NewDecoder(file)
	currencySymbols := CurrencySymbols{}
	err := decoder.Decode(&currencySymbols)
	if err != nil {
		fmt.Println("error:", err)
	}
	for _, v := range currencySymbols.Symbolscur {
		go GetHitBtcdata(v)
	}
	http.HandleFunc("/currency/", GetCurrencyForID)
	http.HandleFunc("/currency/all", GetAllCurrency)
	log.Fatal(http.ListenAndServe(":8080", nil))
}


