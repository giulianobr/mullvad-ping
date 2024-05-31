package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"

	probing "github.com/prometheus-community/pro-bing"
)

const URL = "https://api.mullvad.net/www/relays/all/"

type ServerInfo struct {
	Active           bool          `json:"active"`
	CityCode         string        `json:"city_code"`
	CityName         string        `json:"city_name"`
	CountryCode      string        `json:"country_code"`
	CountryName      string        `json:"country_name"`
	Daita            bool          `json:"daita"`
	Fqdn             string        `json:"fqdn"`
	Hostname         string        `json:"hostname"`
	Ipv4AddrIn       string        `json:"ipv4_addr_in"`
	Ipv6AddrIn       string        `json:"ipv6_addr_in"`
	MultihopPort     int           `json:"multihop_port"`
	NetworkPortSpeed int           `json:"network_port_speed"`
	Owned            bool          `json:"owned"`
	Provider         string        `json:"provider"`
	Pubkey           string        `json:"pubkey"`
	SocksName        string        `json:"socks_name"`
	SocksPort        int           `json:"socks_port"`
	StatusMessages   []interface{} `json:"status_messages"`
	Stboot           bool          `json:"stboot"`
	Type             string        `json:"type"`
	List             []float64
	Last             float64
}

type ByLast []*ServerInfo

func (a ByLast) Len() int           { return len(a) }
func (a ByLast) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByLast) Less(i, j int) bool { return a[i].Last < a[j].Last }

func (s *ServerInfo) Ping() (float64, error) {
	fmt.Println("Pinging server", s.Fqdn)
	pinger, err := probing.NewPinger(s.Fqdn)
	pinger.Timeout = 5 * time.Second
	if err != nil {
		panic(err)
	}
	pinger.Count = 3
	err = pinger.Run() // Blocks until finished.
	if err != nil {
		panic(err)
	}
	stats := pinger.Statistics()
	r := stats.AvgRtt.Seconds() * 1000 // Convert time.Duration to milliseconds as float64
	//fmt.Fprintf(os.Stdout, "Server: %s, avgrtt: %f \n", []any{s.Hostname, r}...)

	s.List = append(s.List, r)
	s.Last = float64(stats.MinRtt.Seconds() * 1000)
	return s.Last, nil
}

var serversInfo []*ServerInfo
var activeWireGuard []*ServerInfo

func main() {
	fmt.Println("Fetching all server from Mullvad API ...")

	response, err := http.Get(URL)
	if err != nil {
		fmt.Errorf("Request Error: %v", err.Error())
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.Reader(response.Body))

	jsonErr := json.Unmarshal(body, &serversInfo)
	fmt.Printf("There are %d servers\n\n", len(serversInfo))
	if jsonErr != nil {
		fmt.Println(jsonErr)
	} else {
		for i := range serversInfo {
			if serversInfo[i].Type == "wireguard" && serversInfo[i].Active {
				activeWireGuard = append(activeWireGuard, serversInfo[i])
			}
		}
	}
	fmt.Printf("There are %d active Wireguard servers\n\n", len(activeWireGuard))

	var wg sync.WaitGroup
	wg.Add(len(activeWireGuard) - 1)

	for _, server := range activeWireGuard {
		time.Sleep(100 * time.Millisecond)

		go func(wg *sync.WaitGroup, server *ServerInfo) {
			for i := 0; i < 3; i++ {
				_, err := server.Ping()
				if err != nil {
					fmt.Println("Error on ping", server.Fqdn, err.Error())
					continue
				}

				break
			}

			wg.Done()
		}(&wg, server)
	}

	wg.Wait()

	fmt.Println("------------------------------------------")
	fmt.Println("------------------------------------------")

	sort.Sort(ByLast(activeWireGuard))
	printed := 0
	fmt.Println("Printing the top 10:")
	for _, server := range activeWireGuard {
		if server.Last < 1 {
			continue
		}

		fmt.Println(server.Fqdn, "\t", server.Last, "ms")
		fmt.Println("------------------------------------------")

		if printed >= 9 {
			break
		}

		printed++
	}
}
